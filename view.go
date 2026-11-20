package main

import (
	"cmp"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// View models: everything the template needs, already resolved.

type PageData struct {
	BaseURL   string
	Me        PersonView
	Authored  []RowView
	Reviewing []RowView
	Error     string
	FetchedAt time.Time
	Refresh   int // seconds between client refreshes
}

type RowView struct {
	Title    string
	URL      string
	Ref      string // "!123"
	Draft    bool
	Project  string
	ProjURL  string
	People   []PersonView
	Labels   []LabelView
	Pipeline *PipelineView
	Updated  time.Time
	MyReview string // review state of the current user on this MR (reviewing list only)

	Target    string // target branch, only set when it isn't the project's default branch
	TargetURL string

	Blockers       []BlockerView // unmerged MRs this one depends on
	HiddenBlockers int           // blockers the user can't see

	Approval      string // "approved" (rules satisfied), "partial" (some approvals, more needed) or ""
	ApprovalCount string // "1/2" for partial approvals
	ApprovalTitle string // tooltip: who approved
}

type BlockerView struct {
	Ref   string // "!93", or "Project!93" when in another project
	URL   string
	Title string
}

type PersonView struct {
	Name     string
	Username string
	URL      string
	Avatar   string
	Initials string
	Hue      int    // fallback avatar color
	State    string // approved | changes | reviewed | ""
}

type LabelView struct {
	Title     string
	Scope     string // for scoped labels "Scope::Value"
	Value     string
	Color     string
	TextColor string
}

type PipelineView struct {
	Status string // lowercase status, used as CSS class and icon key
	Text   string
	URL    string
}

var draftPrefix = regexp.MustCompile(`(?i)^\s*(\[draft\]|\(draft\)|draft:|draft -)\s*`)

func buildPage(base string, d *Dashboard) PageData {
	p := PageData{BaseURL: base}
	if d == nil {
		return p
	}
	p.Me = person(base, User{Username: d.Me.Username, Name: d.Me.Name, AvatarURL: d.Me.AvatarURL, WebURL: d.Me.WebURL})
	for _, mr := range d.Authored {
		p.Authored = append(p.Authored, row(base, mr, mr.Reviewers.Nodes, ""))
	}
	for _, mr := range d.Reviewing {
		people := mr.Assignees.Nodes
		if len(people) == 0 {
			people = []User{mr.Author} // unassigned: the author is the next best thing
		}
		my := ""
		for _, r := range mr.Reviewers.Nodes {
			if r.Username == d.Me.Username {
				my = reviewState(r)
			}
		}
		if approvedBy(mr)[d.Me.Username] {
			my = "approved"
		} else if my == "approved" {
			my = "reviewed"
		}
		p.Reviewing = append(p.Reviewing, row(base, mr, people, my))
	}
	return p
}

func row(base string, mr MergeRequest, people []User, myReview string) RowView {
	r := RowView{
		Title:    draftPrefix.ReplaceAllString(mr.Title, ""),
		URL:      mr.WebURL,
		Ref:      "!" + mr.IID,
		Draft:    mr.Draft,
		Project:  mr.Project.Name,
		ProjURL:  mr.Project.WebURL,
		Updated:  mr.UpdatedAt,
		MyReview: myReview,
	}
	if repo := mr.Project.Repository; repo != nil && repo.RootRef != "" && mr.TargetBranch != repo.RootRef {
		r.Target = mr.TargetBranch
		r.TargetURL = mr.Project.WebURL + "/-/tree/" + escapeBranch(mr.TargetBranch)
	}

	for _, b := range mr.Blocking.Visible {
		if b.State == "merged" { // a merged dependency no longer blocks
			continue
		}
		ref := "!" + b.IID
		if b.Project.WebURL != mr.Project.WebURL {
			ref = b.Project.Name + ref
		}
		title := b.Title
		if b.State == "closed" {
			title += " (closed)"
		}
		r.Blockers = append(r.Blockers, BlockerView{Ref: ref, URL: b.WebURL, Title: title})
	}
	r.HiddenBlockers = mr.Blocking.HiddenCount

	approvers := approvedBy(mr)
	for _, u := range people {
		pv := person(base, u)
		// The MR's approver list is authoritative; per-reviewer state can lag.
		if approvers[u.Username] {
			pv.State = "approved"
		} else if pv.State == "approved" {
			pv.State = "reviewed"
		}
		r.People = append(r.People, pv)
	}

	var names []string
	for _, u := range mr.ApprovedBy.Nodes {
		names = append(names, cmp.Or(u.Name, u.Username))
	}
	switch {
	case mr.Approved && (len(names) > 0 || mr.ApprovalsRequired > 0):
		r.Approval = "approved"
		r.ApprovalTitle = "Approved"
	case len(names) > 0:
		r.Approval = "partial"
		given := mr.ApprovalsRequired - mr.ApprovalsLeft
		r.ApprovalCount = fmt.Sprintf("%d/%d", given, mr.ApprovalsRequired)
		r.ApprovalTitle = fmt.Sprintf("%d of %d required approvals", given, mr.ApprovalsRequired)
	}
	if len(names) > 0 {
		r.ApprovalTitle += " by " + strings.Join(names, ", ")
	}

	// GitLab doesn't guarantee label order; sort so pills never shuffle.
	labels := append([]Label(nil), mr.Labels.Nodes...)
	sort.SliceStable(labels, func(i, j int) bool {
		a, b := strings.ToLower(labels[i].Title), strings.ToLower(labels[j].Title)
		if a != b {
			return a < b
		}
		return labels[i].Title < labels[j].Title
	})
	for _, l := range labels {
		lv := LabelView{Title: l.Title, Color: l.Color, TextColor: l.TextColor}
		if i := strings.LastIndex(l.Title, "::"); i > 0 {
			lv.Scope, lv.Value = l.Title[:i], l.Title[i+2:]
		}
		r.Labels = append(r.Labels, lv)
	}

	if mr.Pipeline != nil {
		pv := &PipelineView{Status: strings.ToLower(mr.Pipeline.Status), URL: absURL(base, mr.Pipeline.Path)}
		if mr.Pipeline.DetailedStatus != nil {
			pv.Text = mr.Pipeline.DetailedStatus.Text
		}
		if pv.Text == "" {
			pv.Text = strings.ReplaceAll(pv.Status, "_", " ")
		}
		r.Pipeline = pv
	}
	return r
}

// escapeBranch escapes each path segment but keeps the slashes, so
// "feature/x y" becomes "feature/x%20y" as GitLab expects.
func escapeBranch(b string) string {
	parts := strings.Split(b, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func approvedBy(mr MergeRequest) map[string]bool {
	m := make(map[string]bool, len(mr.ApprovedBy.Nodes))
	for _, u := range mr.ApprovedBy.Nodes {
		m[u.Username] = true
	}
	return m
}

func person(base string, u User) PersonView {
	name := u.Name
	if name == "" {
		name = u.Username
	}
	return PersonView{
		Name:     name,
		Username: u.Username,
		URL:      absURL(base, u.WebURL),
		Avatar:   absURL(base, u.AvatarURL),
		Initials: initials(name),
		Hue:      hue(u.Username),
		State:    reviewState(u),
	}
}

func reviewState(u User) string {
	if u.Interaction == nil {
		return ""
	}
	if u.Interaction.Approved {
		return "approved"
	}
	switch u.Interaction.ReviewState {
	case "APPROVED":
		return "approved"
	case "REQUESTED_CHANGES":
		return "changes"
	case "REVIEWED":
		return "reviewed"
	}
	return ""
}

func absURL(base, u string) string {
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return base + u
}

func initials(name string) string {
	var out []rune
	for _, f := range strings.FieldsFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '.' || r == '_' || r == '-' }) {
		out = append(out, unicode.ToUpper([]rune(f)[0]))
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

func hue(s string) int {
	h := 0
	for _, c := range s {
		h = (h*31 + int(c)) % 360
	}
	return h
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
	}
}

var reviewStateText = map[string]string{
	"approved": "Approved",
	"changes":  "Requested changes",
	"reviewed": "Reviewed",
}
