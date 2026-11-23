package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

//go:embed web
var webFS embed.FS

func main() {
	var (
		gitlabURL = flag.String("gitlab", envOr("GITLAB_URL", "https://gitlab.com"), "GitLab base URL (env GITLAB_URL)")
		listen    = flag.String("listen", envOr("DASHBOARD_LISTEN", "127.0.0.1:8080"), "address to listen on (env DASHBOARD_LISTEN)")
		refresh   = flag.Duration("refresh", 60*time.Second, "how often the page refreshes itself")
	)
	flag.Parse()

	base := strings.TrimRight(*gitlabURL, "/")
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		log.Fatalf("invalid GitLab URL %q", *gitlabURL)
	}

	token, err := findToken(u.Host)
	if err != nil {
		log.Fatal(err)
	}

	gl := &GitLab{BaseURL: base, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}
	ttl := max(*refresh/2, 15*time.Second)
	c := &cache{gl: gl, ttl: ttl}

	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"ago":          timeAgo,
		"iso":          func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") },
		"reviewText":   func(s string) string { return reviewStateText[s] },
		"pipelineIcon": pipelineIcon,
		"reviewIcon":   reviewIcon,
		"ciClass":      pipelineClass,
		"branchIcon":   func() template.HTML { return branchIcon },
		"blockedIcon":  func() template.HTML { return blockedIcon },
		"clockIcon":    func() template.HTML { return clockIcon },
		"dict": func(kv ...any) map[string]any {
			m := make(map[string]any, len(kv)/2)
			for i := 0; i+1 < len(kv); i += 2 {
				m[kv[i].(string)] = kv[i+1]
			}
			return m
		},
	}).ParseFS(webFS, "web/*.html"))

	static, _ := fs.Sub(webFS, "web/static")
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		d, fetchedAt, ferr := c.get(r.Context(), r.URL.Query().Has("force"))
		page := buildPage(base, d)
		page.FetchedAt = fetchedAt
		page.Refresh = int(refresh.Seconds())
		if ferr != nil {
			page.Error = ferr.Error()
		}
		name := "index.html"
		if r.URL.Query().Has("partial") {
			name = "content"
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := tmpl.ExecuteTemplate(w, name, page); err != nil {
			log.Printf("render: %v", err)
		}
	})

	log.Printf("GitLab dashboard for %s on http://%s", base, *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

// cache keeps the last successful fetch so the page stays useful if GitLab
// hiccups, and so reloads don't hammer the API.
type cache struct {
	gl  *GitLab
	ttl time.Duration

	mu        sync.Mutex
	data      *Dashboard
	fetchedAt time.Time
	lastErr   error
	errAt     time.Time
}

func (c *cache) get(ctx context.Context, force bool) (*Dashboard, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fresh := c.data != nil && time.Since(c.fetchedAt) < c.ttl
	recentErr := c.lastErr != nil && time.Since(c.errAt) < 10*time.Second
	if (fresh || recentErr) && !force {
		return c.data, c.fetchedAt, c.lastErr
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	d, err := c.gl.FetchDashboard(ctx)
	if err != nil {
		log.Printf("fetch: %v", err)
		c.lastErr, c.errAt = err, time.Now()
		return c.data, c.fetchedAt, err
	}
	c.data, c.fetchedAt, c.lastErr = d, time.Now(), nil
	return c.data, c.fetchedAt, nil
}

// findToken looks for a GitLab token in the environment, then asks glab.
func findToken(host string) (string, error) {
	for _, k := range []string{"GITLAB_TOKEN", "GITLAB_ACCESS_TOKEN", "GLAB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v, nil
		}
	}
	if _, err := exec.LookPath("glab"); err == nil {
		out, err := exec.Command("glab", "config", "get", "token", "--host", host).Output()
		if tok := strings.TrimSpace(string(out)); err == nil && tok != "" {
			return tok, nil
		}
	}
	return "", errors.New("no GitLab token found: set GITLAB_TOKEN (scope read_api) or log in with `glab auth login --hostname " + host + "`")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
