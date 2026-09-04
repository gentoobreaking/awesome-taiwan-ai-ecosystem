package classify

import (
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

// ScoreResult holds the result of scoring a server.
type ScoreResult struct {
	Score     float64
	Level     string
	Evidence  []models.Evidence
}

// Score computes the Taiwan relevance score for an MCPServer (§17).
// Returns the relevance level (T0-T5) and evidence for each scoring rule.
//
// Scoring rules (§17):
//   - official Taiwan domain match: +40 (evidence type=official_domain)
//   - Taiwan government API detected: +40 (evidence type=official_gov_api)
//   - Taiwan financial API detected: +35 (evidence type=taiwan_financial_api)
//   - Taiwan-specific dataset detected: +30 (evidence type=taiwan_dataset)
//   - Taiwan-specific keyword found: +20 (evidence type=repository_keyword)
//   - Taiwan language detected: +15 (evidence type=taiwan_language)
//   - Taiwan company/service detected: +15 (evidence type=taiwan_company)
//   - README Taiwan mention: +5 (evidence type=readme_mention)
func Score(server *models.MCPServer) ScoreResult {
	var evidence []models.Evidence
	var score float64

	// Collect text fields for keyword matching
	texts := collectTextFields(server)

	// --- official domain match: +40 (§17) ---
	domains := MatchOfficialDomains(collectURLs(server))
	for _, d := range uniqueStrings(domains) {
		score += 40
		evidence = append(evidence, models.Evidence{
			Type:        "official_domain",
			Source:      "repository",
			Location:    d,
			MatchedText: d,
			Rule:        "official_domain_match",
			Score:       40,
			Confidence:  1.0,
			Timestamp:   time.Now().UTC(),
			ContentHash: contentHash(d),
		})
	}

	// --- Taiwan financial API: +35 (§17) ---
	finURLs := MatchDataSource(collectURLs(server))
	for _, ds := range uniqueStrings(finURLs) {
		score += 35
		evidence = append(evidence, models.Evidence{
			Type:        "taiwan_financial_api",
			Source:      "repository",
			Location:    ds,
			MatchedText: ds,
			Rule:        "financial_api_detected",
			Score:       35,
			Confidence:  1.0,
			Timestamp:   time.Now().UTC(),
			ContentHash: contentHash(ds),
	})
	}

	// --- Taiwan keyword match: +20 (§17) ---
	matches := MatchTaiwanKeywords(texts)
	for _, m := range matches {
		pts := keywordScore(m.Category)
		score += pts
		evidence = append(evidence, models.Evidence{
			Type:        "repository_keyword",
			Source:      "repository",
			Location:    m.Keyword,
			MatchedText: m.Keyword,
			Rule:        "taiwan_keyword",
			Score:       pts,
			Confidence:  1.0,
			Timestamp:   time.Now().UTC(),
			ContentHash: contentHash(m.Keyword),
		})
	}

	level := ThresholdToLevel(score)

	return ScoreResult{
		Score:    score,
		Level:    level,
		Evidence: evidence,
	}
}

func keywordScore(category string) float64 {
	switch category {
	case "finance":
		return 20
	case "government":
		return 15
	case "language":
		return 15
	case "company_service":
		return 15
	default:
		return 5
	}
}

// ThresholdToLevel maps a score to its Taiwan relevance level (§17).
//   - score >= 70 → T5
//   - score >= 55 → T4
//   - score >= 40 → T3
//   - score >= 20 → T2
//   - score >= 5 → T1
//   - score < 5 → T0
func ThresholdToLevel(score float64) string {
	switch {
	case score >= 70:
		return "T5"
	case score >= 55:
		return "T4"
	case score >= 40:
		return "T3"
	case score >= 20:
		return "T2"
	case score >= 5:
		return "T1"
	default:
		return "T0"
	}
}

func collectTextFields(server *models.MCPServer) []string {
	var texts []string
	texts = append(texts, server.Name, server.Description, server.Repository.Name,
		server.Repository.Owner, server.Repository.Language, server.Repository.License)
	for _, t := range server.Repository.Topics {
		texts = append(texts, t)
	}
	for _, ds := range server.DataSources {
		texts = append(texts, ds.Name, ds.URL)
	}
	return texts
}

func collectURLs(server *models.MCPServer) []string {
	var urls []string
	urls = append(urls, server.Repository.URL, server.Repository.Homepage)
	for _, ep := range server.Endpoints {
		urls = append(urls, ep.URL)
	}
	for _, ds := range server.DataSources {
		urls = append(urls, ds.URL)
	}
	return urls
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func contentHash(s string) string {
	return hashFNV(s)
}

func hashFNV(s string) string {
	h := uint32(2166136261)
	for _, c := range s {
		h ^= uint32(c)
		h *= 16777619
	}
	return fmtUint32Hex(h)
}

func fmtUint32Hex(v uint32) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		buf[i] = hex[v&0xf]
		v >>= 4
	}
	return string(buf)
}
