package mcpserversorg

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

func TestName(t *testing.T) {
	a := New()
	if a.Name() != "mcpserversorg" {
		t.Fatalf("expected mcpserversorg, got %s", a.Name())
	}
}

func TestDiscover_SitemapParsingAndDedup(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			base := "http://" + r.Host
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s/sitemaps/servers/1.xml</loc></sitemap>
  <sitemap><loc>%s/sitemaps/servers/2.xml</loc></sitemap>
  <sitemap><loc>%s/sitemaps/static.xml</loc></sitemap>
</sitemapindex>`, base, base, base)
		case "/sitemaps/servers/1.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://mcpservers.org/servers/foo-mcp</loc></url>
  <url><loc>https://mcpservers.org/en/servers/foo-mcp</loc></url>
  <url><loc>https://mcpservers.org/servers/bar-mcp</loc></url>
</urlset>`)
		case "/sitemaps/servers/2.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://mcpservers.org/zh-TW/servers/bar-mcp</loc></url>
  <url><loc>https://mcpservers.org/servers/baz-mcp</loc></url>
  <url><loc>https://mcpservers.org/es/servers/baz-mcp</loc></url>
</urlset>`)
		case "/sitemaps/static.xml":
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprint(w, `<?xml version="1.0"?><urlset></urlset>`)
		default:
			if strings.Contains(r.URL.Path, "/servers/") {
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `<html><head><meta name="description" content="Test MCP server"></head><body><article><p>Hello world</p></article><a href="https://github.com/owner/foo-mcp">GitHub</a></body></html>`)
				return
			}
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	a := &Adapter{
		SitemapURL: server.URL + "/sitemap.xml",
		BaseURL:    server.URL,
		HTTP:       server.Client(),
	}

	candidates, err := a.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("expected 3 unique candidates, got %d: %+v", len(candidates), candidates)
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if c.Source != "mcpserversorg" {
			t.Errorf("unexpected source %s", c.Source)
		}
		if seen[c.Name] {
			t.Errorf("duplicate slug %s", c.Name)
		}
		seen[c.Name] = true
		if c.RawMetadata["sitemap_loc"] == nil {
			t.Errorf("missing sitemap_loc for %s", c.Name)
		}
		if c.SourceURL == "" {
			t.Errorf("empty SourceURL for %s", c.Name)
		}
		if c.DiscoveredAt.IsZero() {
			t.Errorf("DiscoveredAt not set for %s", c.Name)
		}
		_ = time.Now()
	}
	for _, want := range []string{"foo-mcp", "bar-mcp", "baz-mcp"} {
		if !seen[want] {
			t.Errorf("missing expected slug %s", want)
		}
	}
}

func TestCanonicalSlug(t *testing.T) {
	tests := []struct {
		loc  string
		want string
	}{
		{"https://mcpservers.org/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/en/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/zh-TW/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/pt-BR/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/es/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/zh-CN/servers/my-mcp", "my-mcp"},
		{"https://mcpservers.org/servers/foo/bar", "foo"},
		{"https://mcpservers.org/other/path", ""},
	}
	for _, tc := range tests {
		got := canonicalSlug(tc.loc)
		if got != tc.want {
			t.Errorf("canonicalSlug(%q)=%q want %q", tc.loc, got, tc.want)
		}
	}
}

func TestFetch_ExtractGitHub(t *testing.T) {
	html := `<html><head><meta name="description" content="A test MCP for Taiwan"></head><body>
<article><p>Taiwan stock data MCP</p></article>
<a href="https://github.com/taiwan/mcp-server">GitHub</a>
<a href="https://example.com/home">Homepage</a>
</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	a := &Adapter{HTTP: server.Client()}
	cand := models.RawCandidate{
		Source:    "mcpserversorg",
		SourceURL: server.URL + "/servers/taiwan-mcp",
		Name:      "taiwan-mcp",
	}
	rec, err := a.Fetch(context.Background(), cand)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if rec.Repository.URL == "" || rec.Repository.URL != "https://github.com/taiwan/mcp-server" {
		t.Fatalf("expected github URL https://github.com/taiwan/mcp-server, got %+v", rec.Repository)
	}
	if !strings.Contains(rec.Readme, "A test MCP for Taiwan") {
		t.Errorf("readme should contain meta description, got %q", rec.Readme)
	}
	if !strings.Contains(rec.Readme, "Taiwan stock data MCP") {
		t.Errorf("readme should contain article text, got %q", rec.Readme)
	}
}

func TestFetch_FallbackHomepage(t *testing.T) {
	html := `<html><head></head><body>
<article><p>No github here</p></article>
<a href="https://example.com/homepage">Home</a>
</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	a := &Adapter{HTTP: server.Client()}
	cand := models.RawCandidate{
		Source:    "mcpserversorg",
		SourceURL: server.URL + "/servers/no-github",
		Name:      "no-github",
	}
	rec, err := a.Fetch(context.Background(), cand)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	if rec.Repository.URL != "" {
		t.Errorf("expected no repository, got %+v", rec.Repository)
	}
	if rec.HomepageURL != "https://example.com/homepage" {
		t.Errorf("expected homepage fallback, got %q", rec.HomepageURL)
	}
}

func TestInterfaceImplementation(t *testing.T) {
	var _ = func() {
		var a interface{} = New()
		if _, ok := a.(interface{ Name() string }); !ok {
			t.Error("Adapter must implement Name()")
		}
	}
	if New().Name() != "mcpserversorg" {
		t.Error("Name mismatch")
	}
}
