package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitLab is a minimal GraphQL client for a single GitLab instance.
type GitLab struct {
	BaseURL string // e.g. https://gitlab.com (no trailing slash)
	Token   string
	HTTP    *http.Client
}

const dashboardQuery = `
fragment UserFields on User {
  username
  name
  avatarUrl
  webUrl
}

fragment MR on MergeRequest {
  iid
  title
  webUrl
  draft
  createdAt
  updatedAt
  sourceBranch
  sourceProject { webUrl }
  targetBranch
  project { name webUrl repository { rootRef } }
  author { ...UserFields }
  assignees { nodes { ...UserFields } }
  reviewers {
    nodes {
      ...UserFields
      mergeRequestInteraction { reviewState approved }
    }
  }
  blockingMergeRequests {
    hiddenCount
    visibleMergeRequests { iid state title webUrl project { name webUrl } }
  }
  approved
  approvalsRequired
  approvalsLeft
  approvedBy { nodes { username name } }
  labels(first: 50) { nodes { title color textColor } }
  headPipeline {
    status
    path
    detailedStatus { text }
  }
}

query Dashboard {
  currentUser {
    username
    name
    avatarUrl
    webUrl
    mrs: %s(state: opened, sort: CREATED_DESC, first: 100) {
      nodes { ...MR }
    }
  }
}`

type User struct {
	Username    string `json:"username"`
	Name        string `json:"name"`
	AvatarURL   string `json:"avatarUrl"`
	WebURL      string `json:"webUrl"`
	Interaction *struct {
		ReviewState string `json:"reviewState"`
		Approved    bool   `json:"approved"`
	} `json:"mergeRequestInteraction,omitempty"`
}

type Label struct {
	Title     string `json:"title"`
	Color     string `json:"color"`
	TextColor string `json:"textColor"`
}

type Pipeline struct {
	Status         string `json:"status"`
	Path           string `json:"path"`
	DetailedStatus *struct {
		Text string `json:"text"`
	} `json:"detailedStatus"`
}

type MergeRequest struct {
	IID           string    `json:"iid"`
	Title         string    `json:"title"`
	WebURL        string    `json:"webUrl"`
	Draft         bool      `json:"draft"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	SourceBranch  string    `json:"sourceBranch"`
	SourceProject *struct {
		WebURL string `json:"webUrl"`
	} `json:"sourceProject"` // differs from Project for forks; nil if the fork is gone
	TargetBranch string `json:"targetBranch"`
	Project      struct {
		Name       string `json:"name"`
		WebURL     string `json:"webUrl"`
		Repository *struct {
			RootRef string `json:"rootRef"` // default branch
		} `json:"repository"`
	} `json:"project"`
	Author    User                    `json:"author"`
	Assignees struct{ Nodes []User }  `json:"assignees"`
	Reviewers struct{ Nodes []User }  `json:"reviewers"`
	Labels    struct{ Nodes []Label } `json:"labels"`
	Pipeline  *Pipeline               `json:"headPipeline"`

	Blocking struct {
		HiddenCount int `json:"hiddenCount"`
		Visible     []struct {
			IID     string `json:"iid"`
			State   string `json:"state"`
			Title   string `json:"title"`
			WebURL  string `json:"webUrl"`
			Project struct {
				Name   string `json:"name"`
				WebURL string `json:"webUrl"`
			} `json:"project"`
		} `json:"visibleMergeRequests"`
	} `json:"blockingMergeRequests"`

	// Approval state of the MR as a whole. approvedBy is authoritative;
	// a reviewer's mergeRequestInteraction can lag behind it.
	Approved          bool                   `json:"approved"`
	ApprovalsRequired int                    `json:"approvalsRequired"`
	ApprovalsLeft     int                    `json:"approvalsLeft"`
	ApprovedBy        struct{ Nodes []User } `json:"approvedBy"`
}

type Dashboard struct {
	Me struct {
		Username  string `json:"username"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatarUrl"`
		WebURL    string `json:"webUrl"`
	}
	Authored  []MergeRequest
	Reviewing []MergeRequest
}

// FetchDashboard loads both lists. Each list is its own GraphQL request
// because together they exceed GitLab's per-query complexity limit (250).
func (g *GitLab) FetchDashboard(ctx context.Context) (*Dashboard, error) {
	type result struct {
		me  *currentUser
		err error
	}
	authored, reviewing := make(chan result, 1), make(chan result, 1)
	go func() { me, err := g.fetchList(ctx, "authoredMergeRequests"); authored <- result{me, err} }()
	go func() { me, err := g.fetchList(ctx, "reviewRequestedMergeRequests"); reviewing <- result{me, err} }()
	a, r := <-authored, <-reviewing
	if a.err != nil {
		return nil, a.err
	}
	if r.err != nil {
		return nil, r.err
	}

	d := &Dashboard{Authored: a.me.MRs.Nodes, Reviewing: r.me.MRs.Nodes}
	d.Me.Username, d.Me.Name, d.Me.AvatarURL, d.Me.WebURL = a.me.Username, a.me.Name, a.me.AvatarURL, a.me.WebURL
	return d, nil
}

type currentUser struct {
	Username  string `json:"username"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	WebURL    string `json:"webUrl"`
	MRs       struct {
		Nodes []MergeRequest `json:"nodes"`
	} `json:"mrs"`
}

func (g *GitLab) fetchList(ctx context.Context, field string) (*currentUser, error) {
	body, _ := json.Marshal(map[string]string{"query": fmt.Sprintf(dashboardQuery, field)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/api/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.Token)

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting GitLab: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("reading GitLab response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab returned %s: %s", resp.Status, truncate(string(raw), 300))
	}

	var out struct {
		Data struct {
			CurrentUser *currentUser `json:"currentUser"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding GitLab response: %w", err)
	}
	if len(out.Errors) > 0 {
		msgs := make([]string, len(out.Errors))
		for i, e := range out.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("GitLab GraphQL error: %s", strings.Join(msgs, "; "))
	}
	if out.Data.CurrentUser == nil {
		return nil, fmt.Errorf("GitLab did not return a current user; is the token valid?")
	}
	return out.Data.CurrentUser, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
