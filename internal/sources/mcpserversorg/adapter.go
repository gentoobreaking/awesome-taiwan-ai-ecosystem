package mcpserversorg

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/david/awesome-taiwan-mcp/internal/models"
	"github.com/david/awesome-taiwan-mcp/internal/sources"
)

// HTTPClient abstracts HTTP GET for testing.
type HTTPClient interface {
	Get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error)
}

// Adapter implements SourceAdapter for mcpservers.org via Sitemap.
type Adapter struct {
	HTTPClient HTTPClient
	BaseURL    string
	SitemapURL string
	HTTP       *http.Client
}

// New creates a new mcpservers.org adapter.
func New() *Adapter {
	return &Adapter{
		BaseURL:    "https://mcpservers.org",
		SitemapURL: "https://mcpservers.org/sitemap.xml",
		HTTP:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Name() string { return "mcpserversorg" }

var _ sources.SourceAdapter = (*Adapter)(nil)

// sitemapIndex mirrors <sitemapindex><sitemap><loc>
type sitemapIndex struct {
	XMLName  xml.Name `xml:"sitemapindex"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type urlSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// get fetches URL as bytes.
func (a *Adapter) get(ctx context.Context, target string) ([]byte, error) {
	if a.HTTPClient != nil {
		data, status, err := a.HTTPClient.Get(ctx, target, map[string]string{"User-Agent": "github.com/david/awesome-taiwan-mcp/1.0"})
		if err != nil {
			return nil, err
		}
		if status < 200 || status >= 300 {
			return nil, fmt.Errorf("GET %s: status %d", target, status)
		}
		return data, nil
	}
	client := a.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "github.com/david/awesome-taiwan-mcp/1.0")
	req.Header.Set("Accept", "application/xml, text/xml, */*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", target, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Discover implements sitemap-based discovery.
// Steps: GET sitemap.xml → parse sitemapindex → parallel GET sitemaps/servers/1..6.xml (sem=2) → parse urlset → dedup by canonical slug.
func (a *Adapter) Discover(ctx context.Context) ([]models.RawCandidate, error) {
	sitemapURL := a.SitemapURL
	if sitemapURL == "" {
		sitemapURL = a.BaseURL + "/sitemap.xml"
		if a.BaseURL == "" {
			sitemapURL = "https://mcpservers.org/sitemap.xml"
		}
	}
	data, err := a.get(ctx, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("fetch sitemap index: %w", err)
	}
	var idx sitemapIndex
	if err := xml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parse sitemap index: %w", err)
	}
	// Filter sitemaps that contain servers
	var serverSitemaps []string
	for _, sm := range idx.Sitemaps {
		loc := strings.TrimSpace(sm.Loc)
		if loc == "" {
			continue
		}
		// Prefer sitemaps/servers/* or any loc containing /servers/
		if strings.Contains(loc, "/servers/") || strings.Contains(loc, "servers") {
			serverSitemaps = append(serverSitemaps, loc)
		}
	}
	// If none matched but we have sitemaps, try to include 1..6 pattern fallback (defensive)
	if len(serverSitemaps) == 0 {
		for _, sm := range idx.Sitemaps {
			loc := strings.TrimSpace(sm.Loc)
			if loc != "" {
				serverSitemaps = append(serverSitemaps, loc)
			}
		}
	}

	// Parallel fetch with sem=2
	type result struct {
		locs []string
		err  error
	}
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allLocs []string

	for _, smURL := range serverSitemaps {
		smURL := smURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			b, err := a.get(ctx, smURL)
			if err != nil {
				// Log but don't fail entire discover; continue
				return
			}
			var us urlSet
			if err := xml.Unmarshal(b, &us); err != nil {
				return
			}
			var locs []string
			for _, u := range us.URLs {
				loc := strings.TrimSpace(u.Loc)
				if loc != "" {
					locs = append(locs, loc)
				}
			}
			mu.Lock()
			allLocs = append(allLocs, locs...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Dedup by canonical slug (strip locale prefix)
	seen := make(map[string]bool)
	var candidates []models.RawCandidate
	for _, loc := range allLocs {
		slug := canonicalSlug(loc)
		if slug == "" {
			continue
		}
		if seen[slug] {
			continue
		}
		seen[slug] = true
		candidates = append(candidates, models.RawCandidate{
			Source:       "mcpserversorg",
			SourceURL:    loc,
			Name:         slug,
			RawMetadata:  map[string]any{"sitemap_loc": loc},
			DiscoveredAt: time.Now(),
		})
	}
	return candidates, nil
}

// canonicalSlug extracts slug after /servers/ and strips locale prefix.
// e.g. https://mcpservers.org/zh-TW/servers/my-mcp -> my-mcp
//      https://mcpservers.org/servers/my-mcp -> my-mcp
//      https://mcpservers.org/en/servers/my-mcp -> my-mcp
func canonicalSlug(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		// fallback string search
		idx := strings.Index(raw, "/servers/")
		if idx == -1 {
			return ""
		}
		rest := raw[idx+len("/servers/"):]
		rest = strings.Split(rest, "?")[0]
		rest = strings.Split(rest, "#")[0]
		rest = strings.Split(rest, "/")[0]
		return strings.TrimSpace(rest)
	}
	p := u.Path
	idx := strings.Index(p, "/servers/")
	if idx == -1 {
		return ""
	}
	rest := p[idx+len("/servers/"):]
	rest = strings.Trim(rest, "/")
	if rest == "" {
		return ""
	}
	// slug is first segment after /servers/
	parts := strings.Split(rest, "/")
	slug := parts[0]
	// strip query/fragment already handled via URL parsing
	return slug
}

func (a *Adapter) Fetch(ctx context.Context, candidate models.RawCandidate) (*sources.RawRecord, error) {
	target := candidate.SourceURL
	if target == "" {
		return nil, fmt.Errorf("empty SourceURL")
	}
	data, err := a.get(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("fetch detail %s: %w", target, err)
	}
	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	// Extract first github.com/OWNER/REPO
	var repoURL string
	doc.Find(`a[href^="https://github.com/"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
		href, ok := s.Attr("href")
		if !ok {
			return true
		}
		// Normalize href: trim fragments/query, split
		u, err := url.Parse(href)
		if err != nil {
			return true
		}
		if u.Host != "github.com" {
			return true
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 {
			return true
		}
		owner := parts[0]
		repo := parts[1]
		if owner == "" || repo == "" {
			return true
		}
		// Exclude single-char or invalid? Accept all
		// Filter out common non-repo paths like site hosts? Heuristic: repo should not contain "."? Keep simple
		repoURL = fmt.Sprintf("https://github.com/%s/%s", owner, repo)
		return false // break
	})

	// Also try http:// variant
	if repoURL == "" {
		doc.Find(`a[href^="http://github.com/"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			href, _ := s.Attr("href")
			u, err := url.Parse(href)
			if err != nil {
				return true
			}
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) < 2 {
				return true
			}
			repoURL = fmt.Sprintf("https://github.com/%s/%s", parts[0], parts[1])
			return false
		})
	}

	// Fallback homepage: first https link not mcpservers.org and not github.com
	var homepage string
	if repoURL == "" {
		doc.Find(`a[href^="https://"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			href, _ := s.Attr("href")
			if href == "" {
				return true
			}
			if strings.Contains(href, "mcpservers.org") {
				return true
			}
			if strings.Contains(href, "github.com") {
				return true
			}
			// skip nav/internal like # or javascript
			if strings.HasPrefix(href, "https://mcpservers.org") {
				return true
			}
			u, err := url.Parse(href)
			if err != nil || u.Host == "" {
				return true
			}
			homepage = href
			return false
		})
		// Also try http
		if homepage == "" {
			doc.Find(`a[href^="http://"]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
				href, _ := s.Attr("href")
				if strings.Contains(href, "mcpservers.org") || strings.Contains(href, "github.com") {
					return true
				}
				homepage = href
				return false
			})
		}
	}

	// Readme: meta description + article first paragraph
	var readmeParts []string
	if desc, ok := doc.Find(`meta[name="description"]`).Attr("content"); ok {
		desc = strings.TrimSpace(desc)
		if desc != "" {
			readmeParts = append(readmeParts, desc)
		}
	}
	// also try meta property og:description
	if len(readmeParts) == 0 {
		if desc, ok := doc.Find(`meta[property="og:description"]`).Attr("content"); ok {
			desc = strings.TrimSpace(desc)
			if desc != "" {
				readmeParts = append(readmeParts, desc)
			}
		}
	}
	// article first paragraph
	artText := strings.TrimSpace(doc.Find("article p").First().Text())
	if artText == "" {
		artText = strings.TrimSpace(doc.Find("main p").First().Text())
	}
	if artText == "" {
		artText = strings.TrimSpace(doc.Find("p").First().Text())
	}
	if artText != "" {
		readmeParts = append(readmeParts, artText)
	}
	readme := strings.Join(readmeParts, "\n\n")

	// Build record
	rec := &sources.RawRecord{
		Candidate:    candidate,
		Readme:       readme,
		Repository:   nil,
		Manifest:     nil,
		Tools:        nil,
		Resources:    nil,
		Prompts:      nil,
		Endpoints:    nil,
		Transport:    nil,
		PackageFiles: nil,
	}
	if repoURL != "" {
		rec.Candidate.RepositoryURL = repoURL
		rec.Repository = &models.RepositoryInfo{
			URL:  repoURL,
			Host: "github.com",
		}
		// Populate Owner/Name heuristically
		if u, err := url.Parse(repoURL); err == nil {
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) >= 2 {
				rec.Repository.Owner = parts[0]
				rec.Repository.Name = parts[1]
			}
		}
	} else if homepage != "" {
		rec.Candidate.HomepageURL = homepage
	} else {
		// keep original
	}

	return rec, nil
}
