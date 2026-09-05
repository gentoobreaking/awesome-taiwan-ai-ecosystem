package githubrepo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DebugFetchDirectory does a direct API call
func (a *GitHubRepoAdapter) DebugFetchDirectory(ctx context.Context, apiURL string) {
	req, _ := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := a.HTTPClient.Do(ctx, req)
	if err != nil {
		fmt.Printf("HTTP error: %v\n", err)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("DEBUG URL: %s\n", apiURL)
	fmt.Printf("DEBUG Status: %d\n", resp.StatusCode)
	fmt.Printf("DEBUG Body: %s\n", strings.Repeat(".", min(len(body), 200)))
}
