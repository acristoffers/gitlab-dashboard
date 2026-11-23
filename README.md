# gitlab-dashboard

A small, fast GitLab dashboard with just two lists:

1. **My merge requests**: open MRs you authored, with their reviewers.
2. **Review requested**: open MRs where you are a reviewer, with their assignees.

Each row shows the pipeline status, title, project name, labels (GitLab-style
coloured pills, scoped labels split in two, sorted alphabetically so they never
shuffle) and the people involved, with avatars and their review state. You also
see your own review state on MRs you're reviewing. Every link goes to GitLab.

Approved MRs get a green **Approved** pill and a green left edge. Hover over
the pill to see who approved. If a project needs more approvals than it has, the
pill shows a count like `1/2` instead.

After the update time comes the MR's branch. When it merges into something
other than the project's default branch, for example a stacked MR on top of a
feature branch, the target follows as `source → target`. Both link to the
branches on GitLab.

If the MR depends on other MRs that aren't merged yet, a red ⊘ follows with
links to them, e.g. `⊘ !801` or `⊘ AutonomyInterfaces!93` for an MR in another
project. Blockers you don't have access to are counted as `+N`.

The page refreshes itself every minute while visible. Press `r` or click
**Refresh** to force a fresh fetch.

## Running

```sh
go build -o gitlab-dashboard .
./gitlab-dashboard            # http://127.0.0.1:8080
```

Each list is one GraphQL request to GitLab, and the two run in parallel.
Combining them would exceed GitLab's query complexity limit. Results are cached for half
the refresh interval (minimum 15s). If GitLab is unreachable, the last good data
stays on screen under an error banner.

### Configuration

| Flag        | Env                | Default                  |
|-------------|--------------------|--------------------------|
| `-gitlab`   | `GITLAB_URL`       | `https://gitlab.com` |
| `-listen`   | `DASHBOARD_LISTEN` | `127.0.0.1:8080`         |
| `-refresh`  |                    | `60s`                    |

A flag on the command line beats the environment variable, which beats the
default. `--help` always shows the built-in defaults.

**Token:** taken from `GITLAB_TOKEN` (or `GITLAB_ACCESS_TOKEN` / `GLAB_TOKEN`).
Otherwise it falls back to `glab config get token --host <gitlab host>`, so if
`glab` is logged in, nothing else is needed. The token needs the `read_api` scope.

The server talks to GitLab with your token. It listens on localhost by default,
so don't expose it beyond that.

### Avatars

Avatars uploaded to GitLab (`/uploads/...`) are only served to a logged-in
browser session. API tokens don't work for them. If your browser can't load one,
the dashboard shows coloured initials instead. Gravatar avatars always load.

Being logged in to GitLab isn't always enough. Firefox's Total Cookie Protection
keeps GitLab's cookies separate when the page you're on is the dashboard, so the
avatar requests go out logged out and fail. Fix it by turning off Enhanced
Tracking Protection for the dashboard's address: click the shield icon in the
address bar while on the dashboard. This affects only that one site.

## License

Copyright © 2026 Álan Crístoffer

This program is free software: you can redistribute it and/or modify it under
the terms of the GNU General Public License as published by the Free Software
Foundation, either version 3 of the License, or (at your option) any later
version. See [LICENSE](LICENSE).
