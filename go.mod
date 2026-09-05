module github.com/david/awesome-taiwan-mcp

go 1.25.0

require (
	github.com/PuerkitoBio/goquery v1.10.3
	github.com/gofri/go-github-ratelimit/v2 v2.0.2
	github.com/spf13/cobra v1.10.2
	golang.org/x/text v0.41.0
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.58.0
)

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	modernc.org/libc v1.75.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

replace (
	github.com/david/awesome-taiwan-mcp/internal/classify => ./internal/classify
	github.com/david/awesome-taiwan-mcp/internal/config => ./internal/config
	github.com/david/awesome-taiwan-mcp/internal/crawler => ./internal/crawler
	github.com/david/awesome-taiwan-mcp/internal/crawler/run => ./internal/crawler/run
	github.com/david/awesome-taiwan-mcp/internal/dedupe => ./internal/dedupe
	github.com/david/awesome-taiwan-mcp/internal/engines => ./internal/engines
	github.com/david/awesome-taiwan-mcp/internal/evidence => ./internal/evidence
	github.com/david/awesome-taiwan-mcp/internal/export => ./internal/export
	github.com/david/awesome-taiwan-mcp/internal/health => ./internal/health
	github.com/david/awesome-taiwan-mcp/internal/manifest => ./internal/manifest
	github.com/david/awesome-taiwan-mcp/internal/metrics => ./internal/metrics
	github.com/david/awesome-taiwan-mcp/internal/models => ./internal/models
	github.com/david/awesome-taiwan-mcp/internal/normalize => ./internal/normalize
	github.com/david/awesome-taiwan-mcp/internal/observability => ./internal/observability
	github.com/david/awesome-taiwan-mcp/internal/retry => ./internal/retry
	github.com/david/awesome-taiwan-mcp/internal/scoring => ./internal/scoring
	github.com/david/awesome-taiwan-mcp/internal/search => ./internal/search
	github.com/david/awesome-taiwan-mcp/internal/security => ./internal/security
	github.com/david/awesome-taiwan-mcp/internal/sources => ./internal/sources
	github.com/david/awesome-taiwan-mcp/internal/sources/github => ./internal/sources/github
	github.com/david/awesome-taiwan-mcp/internal/sources/githubrepo => ./internal/sources/githubrepo
	github.com/david/awesome-taiwan-mcp/internal/sources/mcpmarket => ./internal/sources/mcpmarket
	github.com/david/awesome-taiwan-mcp/internal/sources/mcpserversorg => ./internal/sources/mcpserversorg
	github.com/david/awesome-taiwan-mcp/internal/sources/registry => ./internal/sources/registry
	github.com/david/awesome-taiwan-mcp/internal/storage => ./internal/storage
	github.com/david/awesome-taiwan-mcp/internal/verify => ./internal/verify
)
