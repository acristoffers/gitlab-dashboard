package main

import (
	"testing"
)

func TestRowLabelsAreSortedAndScoped(t *testing.T) {
	mr := MergeRequest{IID: "7", Title: "Draft: Do the thing", Draft: true}
	mr.Labels.Nodes = []Label{
		{Title: "Type::Bug", Color: "#dc143c"},
		{Title: "backend", Color: "#000"},
		{Title: "Status::Ready", Color: "#8fbc8f"},
	}
	r := row("https://git.example", mr, nil, "")

	if r.Title != "Do the thing" {
		t.Errorf("draft prefix not stripped: %q", r.Title)
	}
	var got []string
	for _, l := range r.Labels {
		got = append(got, l.Title)
	}
	want := []string{"backend", "Status::Ready", "Type::Bug"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("label order = %v, want %v", got, want)
		}
	}
	if r.Labels[1].Scope != "Status" || r.Labels[1].Value != "Ready" {
		t.Errorf("scoped label split wrong: %+v", r.Labels[1])
	}
	if r.Labels[0].Scope != "" {
		t.Errorf("plain label treated as scoped: %+v", r.Labels[0])
	}
}

func TestAbsURL(t *testing.T) {
	base := "https://git.example"
	cases := map[string]string{
		"":                              "",
		"/uploads/a.png":                "https://git.example/uploads/a.png",
		"uploads/a.png":                 "https://git.example/uploads/a.png",
		"https://gravatar.com/avatar/x": "https://gravatar.com/avatar/x",
	}
	for in, want := range cases {
		if got := absURL(base, in); got != want {
			t.Errorf("absURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReviewingUsesMyReviewState(t *testing.T) {
	d := &Dashboard{}
	d.Me.Username = "me"
	mr := MergeRequest{IID: "1", Title: "x", Author: User{Username: "author", Name: "Au Thor"}}
	approved := User{Username: "me", Name: "Me"}
	approved.Interaction = &struct {
		ReviewState string `json:"reviewState"`
		Approved    bool   `json:"approved"`
	}{ReviewState: "REVIEWED", Approved: true}
	mr.Reviewers.Nodes = []User{approved}
	mr.ApprovedBy.Nodes = []User{{Username: "me", Name: "Me"}}
	d.Reviewing = []MergeRequest{mr}

	p := buildPage("https://git.example", d)
	r := p.Reviewing[0]
	if r.MyReview != "approved" {
		t.Errorf("MyReview = %q, want approved", r.MyReview)
	}
	if len(r.People) != 1 || r.People[0].Username != "author" {
		t.Errorf("unassigned MR should fall back to author, got %+v", r.People)
	}
}

func TestApprovalState(t *testing.T) {
	alex := User{Username: "alex", Name: "Alex"}
	// Stale interaction: says unreviewed, but GitLab lists alex as approver.
	alex.Interaction = &struct {
		ReviewState string `json:"reviewState"`
		Approved    bool   `json:"approved"`
	}{ReviewState: "UNREVIEWED"}

	mr := MergeRequest{IID: "1", Title: "x", Approved: true}
	mr.Reviewers.Nodes = []User{alex}
	mr.ApprovedBy.Nodes = []User{{Username: "alex", Name: "Alex"}}
	r := row("https://git.example", mr, mr.Reviewers.Nodes, "")
	if r.Approval != "approved" || r.ApprovalTitle != "Approved by Alex" {
		t.Errorf("approval = %q / %q", r.Approval, r.ApprovalTitle)
	}
	if r.People[0].State != "approved" {
		t.Errorf("approver badge missing: state %q", r.People[0].State)
	}

	partial := MergeRequest{IID: "2", Title: "y", ApprovalsRequired: 2, ApprovalsLeft: 1}
	partial.ApprovedBy.Nodes = []User{{Username: "alex", Name: "Alex"}}
	r = row("https://git.example", partial, nil, "")
	if r.Approval != "partial" || r.ApprovalCount != "1/2" {
		t.Errorf("partial approval = %q %q", r.Approval, r.ApprovalCount)
	}

	none := MergeRequest{IID: "3", Title: "z"}
	if r = row("https://git.example", none, nil, ""); r.Approval != "" {
		t.Errorf("unapproved MR marked %q", r.Approval)
	}
}
