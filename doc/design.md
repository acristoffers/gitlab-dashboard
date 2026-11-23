# Design

This document explains how gitlab-dashboard is built and why. It's for people
working on the code. The [README](../README.md) is for people using it.

## Goals

- **Show only what helps you decide what to do next.** There are two lists (MRs
  you authored, MRs you're reviewing), and each row is dense but readable.
  Anything that doesn't change what you'd do next stays on GitLab, one click
  away.
- **Easy to run.** It's a single binary with no runtime files and no
  third-party Go modules. Everything it serves is compiled in.
- **Doesn't get in the way.** The page stays put while it refreshes, and if
  GitLab goes down it keeps showing the last good data instead of a blank page.

## Layout

| File                   | Responsibility                                                        |
|------------------------|-----------------------------------------------------------------------|
| `main.go`              | Flags and configuration, finding the token, HTTP server, cache.       |
| `gitlab.go`            | GraphQL query and response types, the GitLab client.                  |
| `view.go`              | Turns GitLab data into view models: everything the template needs.    |
| `icons.go`             | Inline SVG icons (pipeline status, review state, branch, clock, blocked). |
| `web/index.html`       | `html/template` page with named blocks (`content`, `section`, `avatar`). |
| `web/static/style.css` | All styling, with light and dark themes using CSS custom properties.  |
| `web/static/app.js`    | Auto-refresh, the refresh button and the `r` shortcut.                |
| `view_test.go`         | Tests for the view-model rules described below.                       |

The layers only go one way: `gitlab.go` knows nothing about HTML, `view.go`
knows nothing about HTTP, and the template contains no logic beyond
conditionals and loops. Anything that needs a decision (what counts as approved,
which blockers to show, whether to show the target branch) is made in `view.go`,
where it can be tested.

## Data flow

```
browser ── GET / or /?partial=1 ──▶ handler ──▶ cache ──(stale?)──▶ GitLab GraphQL
                                       │                                  │
                                       ◀──── buildPage(view models) ◀─────┘
                                       │
                                html/template
```

1. The handler asks the cache for data. `?force` skips the cache.
2. `buildPage` turns the raw `Dashboard` into a `PageData` view model.
3. The template renders either the whole page or, with `?partial=1`, only the
   `content` block.

### GitLab API: GraphQL, one request per list

GraphQL returns everything a row needs in a single request: reviewers with
their review state, label colours, pipeline status, approvals, blockers and
branches. Getting the same from REST would take several requests per MR (REST
list endpoints don't include the pipeline or label colours, for example).

The two lists are **separate requests run concurrently**. A combined query went
over GitLab's per-query complexity limit (250) once approval fields were
added. Splitting them keeps each request well under the limit, leaving room for
new fields. Both requests use the same `MR` fragment, and the query text is a
template where only the connection field changes (`authoredMergeRequests` /
`reviewRequestedMergeRequests`, aliased to `mrs`).

**Whenever you add a field, run the dashboard against a real instance.** Going
over the complexity limit only shows up as a GraphQL error at runtime.

The sort order is `CREATED_DESC`, not "recently updated". Rows don't jump around
when someone comments, so the list stays where your eyes left it. Each list is
capped at 100 MRs and there's no pagination. If one person has more than 100
open MRs, a dashboard won't help them.

### Caching

`cache` in `main.go` holds the last successful fetch:

- **Fresh for half the refresh interval** (minimum 15s). Several tabs, or a
  reload right after an auto-refresh, don't each hit GitLab.
- **Errors are remembered for 10s**, so a failing GitLab isn't hammered by every
  request.
- **On error, the previous data is still returned**, along with the error. The
  page shows a banner above the last good lists. If there has never been a
  successful fetch, the lists are hidden rather than showing a misleading
  "no merge requests".
- A mutex serialises fetches, so concurrent requests can't cause duplicate
  GitLab calls.

### Refreshing in the browser

`app.js` fetches `/?partial=1` and replaces `<main>`'s contents. This was chosen
over `<meta http-equiv="refresh">` because a full page reload loses your scroll
position and flashes the page. The script:

- refreshes on an interval only while the tab is visible, and immediately when
  you return to a tab that has gone stale;
- updates "Synced Xm ago" every few seconds without contacting the server;
- uses `?force=1` for the button and the `r` key, so a manual refresh always
  gets fresh data.

The header, footer and script are outside the refreshed block, so they never
re-render.

## Rules for what a row shows

These rules live in `view.go` and are tested in `view_test.go`.

### Approvals

The MR's `approvedBy` list is **the source of truth**. A reviewer's
`mergeRequestInteraction.approved` / `reviewState` can lag behind it: we saw a
reviewer listed in `approvedBy` whose own interaction still said
`UNREVIEWED`. So:

- an avatar gets the ✓ badge if, and only if, that user is in `approvedBy`;
- an interaction that claims "approved" without being in `approvedBy` is shown
  as "reviewed";
- `reviewState` is still used for "requested changes" and "reviewed", which
  `approvedBy` can't express.

At MR level:

- **Approved**: GitLab's `approved` is true and someone approved (or rules
  require approvals). With 0 required approvals, GitLab reports `approved: false`
  until someone approves, so this matches what people expect.
- **Partial (`1/2`)**: some approvals, but rules still need more.

### Labels

Labels are sorted by case-insensitive title, with an exact-title comparison as a
tie-breaker. GitLab doesn't guarantee an order, and pills that move around
between refreshes are hard to scan. Scoped labels (`Scope::Value`) are split at
the **last** `::` and drawn like GitLab draws them: the key filled with the
label colour, the value outlined.

### Branches

- The **source branch** is always shown. Its link uses `sourceProject.webUrl`,
  so branches from forks link to the fork. If the fork has been deleted,
  `sourceProject` is null and the branch is shown without a link.
- The **target branch** is shown only when it differs from the project's
  default branch (`repository.rootRef`). Merging into the default is the normal
  case and showing it would just be noise. A different target usually means a
  stacked MR, which you want to notice.
- Branch names are escaped segment by segment (`escapeBranch`), so `/` stays a
  path separator in `/-/tree/<branch>` links.

### Blockers

From `blockingMergeRequests` (GitLab Premium MR dependencies):

- **Merged blockers are dropped.** They no longer block anything.
- **Closed blockers are kept.** GitLab keeps the dependent MR blocked until the
  dependency is removed, so it's still something you need to act on. The
  tooltip says "(closed)".
- **Blockers in another project** are shown as `Project!iid`, because a bare
  `!iid` would suggest the wrong project.
- **Blockers you can't see** (`hiddenCount`) are shown as `+N`, so you still know
  the MR is blocked.

Only "blocked by" is shown, never "blocks". What other MRs are waiting on yours
doesn't change what you can do with yours.

### People

- "My merge requests" shows **reviewers**. "Review requested" shows
  **assignees**, falling back to the **author** when nobody is assigned.
- Relative avatar URLs are made absolute against the GitLab base URL.
- Each person also gets initials and a colour derived from a hash of their
  username, shown when their avatar can't load (see below).

### Titles

A `Draft:` / `[Draft]` / `(Draft)` prefix is removed from the title and shown
as a tag instead. GitLab already reports draft status in its own `draft`
field, so the prefix in the title is redundant.

## Avatars and why there's no proxy

Avatars uploaded to GitLab (`/uploads/-/system/user/avatar/...`) are served only
to a **browser session**. We tried `PRIVATE-TOKEN`, `Authorization: Bearer` and
the `private_token` query parameter against a private instance: all returned
401. There is no API endpoint that returns the image itself. So the server
**can't** proxy avatars with a token.

The browser has to load them directly. GitLab's session cookie is
`SameSite=None`, but Firefox's Total Cookie Protection partitions third-party
cookies by top-level site, so requests from the dashboard go out without the
session. The README explains the per-site exception users can turn on.

The fallback is cheap and looks fine: initials sit underneath the `<img>`, and
an `error` listener (in `<head>`, so it's in place before any image fails)
removes broken images. A proxy that uses the user's session cookie was ruled
out: it would need a full-access, expiring credential in the server just for
pictures.

## Templates and rendering

- **`html/template` with `embed`.** It escapes according to context: titles in
  text, URLs in `href`/`src`, label colours in `style`. GitLab data can't inject
  markup, `javascript:` URLs or CSS. A colour that isn't a valid CSS value
  renders as `ZgotmplZ` instead of being injected.
- **Icons are `template.HTML`**, which skips escaping. That's only acceptable
  because the SVG strings are constants in `icons.go` and never include GitLab
  data. Keep it that way: never build an icon from fetched data.
- **`dict`** works around `{{template}}` accepting only one argument, so a
  `section` can receive its title, rows and column name.
- Template functions are kept small and free of side effects. Anything more
  complex belongs in the view model.

## Styling

- One stylesheet, using CSS custom properties for colours. Dark mode redefines
  the variables under `prefers-color-scheme: dark`, so there's only one set of
  rules.
- Status colours (`--ok`, `--bad`, `--run`, `--warn`, `--neutral`) are shared by
  pipeline icons, review badges, the approval pill and the blocker chip, so the
  same colour always means the same thing.
- Label colours come from GitLab through `--lc` / `--lt` custom properties set
  on each pill.
- The row is a 4-column grid. Below 860px it becomes one column, with labels and
  people moved under the title.

## Configuration and token

- **Precedence: flag > environment variable > built-in default.** The
  environment is applied *after* `flag.Parse` (`applyEnv`), only to flags that
  weren't given. Reading the environment while defining a flag would make
  `--help` print the environment value as "the default", which is misleading.
- **Token lookup:** `GITLAB_TOKEN`, `GITLAB_ACCESS_TOKEN`, `GLAB_TOKEN`, then
  `glab config get token --host <host>`. So anyone already using `glab` needs no
  setup.

## Security

- The token never reaches the browser. Only the server talks to the GitLab API.
- The default listen address is `127.0.0.1`. Anyone who can reach the server
  can see what the token can see.
- The server only reads. The query has no mutations, and `read_api` is enough.
- GitLab responses are size-limited (32 MiB), and requests to GitLab time out
  after 30s.

## Packaging

`flake.nix` uses `buildGoModule` with `vendorHash = null`, which works because
there are no third-party modules. Adding a dependency means adding a real
`vendorHash`. Tests run as part of the Nix build. Sources are filtered with
`gitignore.nix`, and like any flake, only files tracked by git are visible to
Nix.

## Testing

`view_test.go` covers the rules above: label order and scoped-label splitting,
draft-prefix stripping, URL handling, approval state (including a reviewer whose
interaction lags behind), partial approvals, target-branch display, source
branch links and blocker filtering. The GraphQL layer isn't mocked. Changes to
the query should be checked against a real instance, mainly for the complexity
limit.
