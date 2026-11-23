//go:build demo

// Made-up merge requests for screenshots. Not part of normal builds:
//
//	go run -tags demo .
//
// Nothing here is real. The data is written as GitLab GraphQL JSON and decoded
// into the same types as live responses, so it goes through the same code.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

func init() { demoData = buildDemo }

type obj = map[string]any

func buildDemo(base string) *Dashboard {
	now := time.Now()
	ago := func(d time.Duration) string { return now.Add(-d).UTC().Format(time.RFC3339) }

	// People. Some use a Gravatar identicon, others fall back to initials.
	user := func(username, name string, identicon bool) obj {
		u := obj{"username": username, "name": name, "webUrl": base + "/" + username}
		if identicon {
			u["avatarUrl"] = fmt.Sprintf("https://secure.gravatar.com/avatar/%x?s=80&d=identicon",
				sha256.Sum256([]byte("gitlab-dashboard-demo/"+username)))
		}
		return u
	}
	me := user("sam.rivera", "Sam Rivera", false)
	priya := user("priya.n", "Priya Natarajan", true)
	marcus := user("mchen", "Marcus Chen", false)
	lena := user("lena.fischer", "Lena Fischer", true)
	tomas := user("toliveira", "Tomás Oliveira", false)
	aisha := user("abello", "Aisha Bello", false)
	jonas := user("jberg", "Jonas Berg", false)

	// reviewer adds GitLab's per-reviewer interaction to a user.
	reviewer := func(u obj, state string) obj {
		r := obj{}
		for k, v := range u {
			r[k] = v
		}
		r["mergeRequestInteraction"] = obj{"reviewState": state, "approved": state == "APPROVED"}
		return r
	}

	label := func(title, color, text string) obj { return obj{"title": title, "color": color, "textColor": text} }
	const dark, light = "#1F1E24", "#FFFFFF"
	var (
		feature   = label("Type::Feature", "#009966", light)
		bug       = label("Type::Bug", "#dc143c", light)
		chore     = label("Type::Chore", "#6699cc", light)
		ready     = label("Status::Ready", "#8fbc8f", dark)
		reviewing = label("Status::Reviewing", "#428bca", light)
		changes   = label("Status::ChangesRequested", "#ed9121", dark)
		high      = label("Priority::High", "#cc338b", light)
		security  = label("security", "#7a1f3d", light)
		frontend  = label("frontend", "#a295d6", dark)
		backend   = label("backend", "#5843ad", light)
	)

	project := func(name, path string) obj {
		return obj{"name": name, "webUrl": base + "/acme/" + path, "repository": obj{"rootRef": "main"}}
	}
	var (
		api     = project("api-gateway", "api-gateway")
		web     = project("web-app", "web-app")
		billing = project("billing-service", "billing-service")
		infra   = project("infrastructure", "infrastructure")
		mobile  = project("mobile-client", "mobile-client")
	)

	type mrSpec struct {
		iid, title     string
		proj           obj
		source, target string
		draft          bool
		updated        time.Duration
		pipeline       string // GitLab status; "" for none
		labels         []obj
		author         obj
		assignees      []obj
		reviewers      []obj
		approvedBy     []obj
		approvalsReq   int
		blockedBy      []obj // MRs from refMR
	}
	refMR := func(iid string, p obj, title string) obj {
		return obj{"iid": iid, "state": "opened", "title": title,
			"webUrl": fmt.Sprintf("%s/-/merge_requests/%s", p["webUrl"], iid), "project": p}
	}
	mr := func(s mrSpec) obj {
		pURL := s.proj["webUrl"].(string)
		target := s.target
		if target == "" {
			target = "main"
		}
		m := obj{
			"iid": s.iid, "title": s.title, "draft": s.draft,
			"webUrl":       fmt.Sprintf("%s/-/merge_requests/%s", pURL, s.iid),
			"createdAt":    ago(s.updated + 6*time.Hour),
			"updatedAt":    ago(s.updated),
			"project":      s.proj,
			"sourceBranch": s.source, "sourceProject": obj{"webUrl": pURL}, "targetBranch": target,
			"author":                s.author,
			"assignees":             obj{"nodes": s.assignees},
			"reviewers":             obj{"nodes": s.reviewers},
			"labels":                obj{"nodes": s.labels},
			"approvedBy":            obj{"nodes": s.approvedBy},
			"approved":              len(s.approvedBy) >= max(1, s.approvalsReq),
			"approvalsRequired":     s.approvalsReq,
			"approvalsLeft":         max(0, s.approvalsReq-len(s.approvedBy)),
			"blockingMergeRequests": obj{"hiddenCount": 0, "visibleMergeRequests": s.blockedBy},
		}
		if s.pipeline != "" {
			m["headPipeline"] = obj{"status": s.pipeline, "path": "/acme/pipelines/1"}
		}
		return m
	}

	authored := []obj{
		mr(mrSpec{iid: "412", title: "Add OAuth2 device flow for CLI login", proj: api,
			source: "feature/device-flow", updated: 12 * time.Minute, pipeline: "RUNNING",
			labels: []obj{reviewing, feature, security}, author: me, assignees: []obj{me},
			reviewers: []obj{reviewer(priya, "REVIEWED"), reviewer(marcus, "UNREVIEWED")}}),
		mr(mrSpec{iid: "415", title: "Store refresh tokens encrypted at rest", proj: api,
			source: "feature/token-crypto", target: "feature/device-flow",
			updated: 40 * time.Minute, pipeline: "PENDING",
			labels: []obj{feature, ready, security}, author: me, assignees: []obj{me},
			reviewers: []obj{reviewer(priya, "UNREVIEWED")},
			blockedBy: []obj{refMR("412", api, "Add OAuth2 device flow for CLI login")}}),
		mr(mrSpec{iid: "417", title: "Draft: Token revocation endpoint", proj: api, draft: true,
			source: "feature/revoke-tokens", target: "feature/token-crypto",
			updated: 2 * time.Hour, labels: []obj{feature}, author: me, assignees: []obj{me},
			reviewers: []obj{reviewer(jonas, "UNREVIEWED")},
			blockedBy: []obj{refMR("415", api, "Store refresh tokens encrypted at rest")}}),
		mr(mrSpec{iid: "88", title: "Fix flaky retry test in the payment webhook handler", proj: billing,
			source: "bugfix/flaky-webhook-retry", updated: 3 * time.Hour, pipeline: "FAILED",
			labels: []obj{bug, changes, backend}, author: me, assignees: []obj{me},
			reviewers: []obj{reviewer(lena, "REQUESTED_CHANGES")}}),
		mr(mrSpec{iid: "1290", title: "Dark mode for the settings page", proj: web,
			source: "feature/settings-dark-mode", updated: 5 * time.Hour, pipeline: "SUCCESS",
			labels: []obj{frontend, feature, ready}, author: me, assignees: []obj{me},
			reviewers:  []obj{reviewer(tomas, "APPROVED"), reviewer(aisha, "APPROVED")},
			approvedBy: []obj{tomas, aisha}}),
		mr(mrSpec{iid: "57", title: "Upgrade Postgres to 17 in staging", proj: infra,
			source: "chore/postgres-17", updated: 26 * time.Hour, pipeline: "SUCCESS",
			labels: []obj{chore, high, ready}, author: me, assignees: []obj{me},
			reviewers:  []obj{reviewer(jonas, "APPROVED"), reviewer(marcus, "UNREVIEWED")},
			approvedBy: []obj{jonas}, approvalsReq: 2}),
		mr(mrSpec{iid: "233", title: "Show an offline banner when sync fails", proj: mobile,
			source: "feature/offline-banner", updated: 50 * time.Hour, pipeline: "CANCELED",
			labels: []obj{frontend, feature, reviewing}, author: me, assignees: []obj{me},
			reviewers: []obj{reviewer(tomas, "UNREVIEWED")},
			blockedBy: []obj{refMR("1290", web, "Dark mode for the settings page")}}),
	}

	reviewList := []obj{
		mr(mrSpec{iid: "409", title: "Rate-limit login attempts per IP", proj: api,
			source: "feature/login-rate-limit", updated: 25 * time.Minute, pipeline: "SUCCESS",
			labels: []obj{reviewing, feature, security}, author: marcus, assignees: []obj{marcus},
			reviewers: []obj{reviewer(me, "UNREVIEWED")}}),
		mr(mrSpec{iid: "86", title: "Move webhook processing to a background worker", proj: billing,
			source: "refactor/webhook-worker", target: "release/2.4", updated: 4 * time.Hour, pipeline: "RUNNING",
			labels: []obj{backend, chore}, author: lena, assignees: []obj{lena},
			reviewers: []obj{reviewer(me, "REVIEWED")}}),
		mr(mrSpec{iid: "1281", title: "Migrate the checkout form to the new design system", proj: web,
			source: "chore/checkout-redesign", updated: 28 * time.Hour, pipeline: "FAILED",
			labels: []obj{changes, chore, frontend}, author: aisha, assignees: []obj{aisha},
			reviewers: []obj{reviewer(me, "REQUESTED_CHANGES")}}),
		mr(mrSpec{iid: "55", title: "Add a Grafana dashboard for queue latency", proj: infra,
			source: "feature/queue-latency", updated: 96 * time.Hour, pipeline: "SUCCESS",
			labels: []obj{feature, high, ready}, author: jonas, assignees: []obj{jonas},
			reviewers:  []obj{reviewer(me, "APPROVED")},
			approvedBy: []obj{me}}),
	}

	var d Dashboard
	decode := func(v any, into any) {
		raw, _ := json.Marshal(v)
		if err := json.Unmarshal(raw, into); err != nil {
			log.Fatalf("demo data: %v", err)
		}
	}
	decode(authored, &d.Authored)
	decode(reviewList, &d.Reviewing)
	decode(me, &d.Me)
	return &d
}
