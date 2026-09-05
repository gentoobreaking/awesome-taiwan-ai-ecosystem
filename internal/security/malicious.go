package security

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RiskLevel represents the risk level of a malicious finding.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "LOW"
	RiskLevelMedium   RiskLevel = "MEDIUM"
	RiskLevelHigh     RiskLevel = "HIGH"
	RiskLevelCritical RiskLevel = "CRITICAL"
)

// MaliciousSignal represents a single detection signal.
type MaliciousSignal struct {
	Type        string  `json:"type"`        // e.g., "readme_entropy", "obfuscation_lua", "account_anomaly"
	Description string  `json:"description"` // human-readable description
	Severity    RiskLevel `json:"severity"`  // severity of this signal
	Evidence    string  `json:"evidence"`    // matched pattern or evidence
	Confidence  float64 `json:"confidence"`  // confidence in this signal (0-1)
}

// MaliciousResult holds the overall malicious detection result.
type MaliciousResult struct {
	RiskLevel  RiskLevel         `json:"risk_level"`  // overall risk level
	Signals    []MaliciousSignal `json:"signals"`     // all detected signals
	Confidence float64           `json:"confidence"`  // overall confidence (0-1)
}

// MaliciousDetector detects malicious repository characteristics.
type MaliciousDetector struct {
	// Configurable thresholds
	EntropyThreshold      float64
	ReadmeSizeThreshold   int
	NonTextRatioThreshold float64
	AccountAgeThreshold   time.Duration
	MinFollowers          int
	MinRepos              int
}

// NewMaliciousDetector creates a new detector with default thresholds.
func NewMaliciousDetector() *MaliciousDetector {
	return &MaliciousDetector{
		EntropyThreshold:      7.0,
		ReadmeSizeThreshold:   100 * 1024, // 100KB
		NonTextRatioThreshold: 0.30,       // 30%
		AccountAgeThreshold:   90 * 24 * time.Hour,
		MinFollowers:          0,
		MinRepos:              5,
	}
}

// Detect analyzes a repository for malicious characteristics.
func (d *MaliciousDetector) Detect(readme string, repoInfo RepositoryInfo) MaliciousResult {
	var signals []MaliciousSignal

	// 1. README entropy check
	if entropySignal := d.checkReadmeEntropy(readme); entropySignal != nil {
		signals = append(signals, *entropySignal)
	}

	// 2. README size anomaly check
	if sizeSignal := d.checkReadmeSize(readme); sizeSignal != nil {
		signals = append(signals, *sizeSignal)
	}

	// 3. Obfuscated code patterns
	if obfSignals := d.checkObfuscationPatterns(readme); len(obfSignals) > 0 {
		signals = append(signals, obfSignals...)
	}

	// 4. Account anomaly check
	if accountSignal := d.checkAccountAnomaly(repoInfo); accountSignal != nil {
		signals = append(signals, *accountSignal)
	}

	// 5. Non-text ratio check
	if nonTextSignal := d.checkNonTextRatio(readme); nonTextSignal != nil {
		signals = append(signals, *nonTextSignal)
	}

	// Calculate overall risk level and confidence
	riskLevel, confidence := d.aggregateRisk(signals)

	return MaliciousResult{
		RiskLevel:  riskLevel,
		Signals:    signals,
		Confidence: confidence,
	}
}

// checkReadmeEntropy checks for high entropy in README (indicating binary/encoded content).
func (d *MaliciousDetector) checkReadmeEntropy(readme string) *MaliciousSignal {
	if len(readme) == 0 {
		return nil
	}

	// Calculate Shannon entropy
	entropy := calculateEntropy(readme)

	if entropy > d.EntropyThreshold {
		// Additional check: is it structured markdown?
		hasStructure := d.hasMarkdownStructure(readme)
		if !hasStructure {
			return &MaliciousSignal{
				Type:        "readme_entropy",
				Description: "README has high Shannon entropy (>7.0) without markdown structure, likely binary/encoded content",
				Severity:    RiskLevelHigh,
				Evidence:    "entropy=" + formatFloat(entropy, 2) + ", length=" + strconv.Itoa(len(readme)),
				Confidence:  0.85,
			}
		}
	}
	return nil
}

// checkReadmeSize checks for abnormally large README without standard structure.
func (d *MaliciousDetector) checkReadmeSize(readme string) *MaliciousSignal {
	if len(readme) > d.ReadmeSizeThreshold {
		hasStructure := d.hasMarkdownStructure(readme)
		if !hasStructure {
			return &MaliciousSignal{
				Type:        "readme_size_anomaly",
				Description: "README exceeds 100KB without standard markdown headings/paragraphs",
				Severity:    RiskLevelMedium,
				Evidence:    "size=" + strconv.Itoa(len(readme)) + " bytes",
				Confidence:  0.75,
			}
		}
	}
	return nil
}

// checkObfuscationPatterns detects obfuscated code patterns in README.
func (d *MaliciousDetector) checkObfuscationPatterns(readme string) []MaliciousSignal {
	var signals []MaliciousSignal

	// Lua VM bytecode patterns
	luaPatterns := []struct {
		pattern     string
		description string
		severity    RiskLevel
	}{
		{`while\s+W\[`, "Lua VM instruction pattern (while W[)", RiskLevelHigh},
		{`0x[0-9A-Fa-f]{2,}`, "Hex byte sequence (potential Lua bytecode)", RiskLevelMedium},
		{`\\[=\\[.*?\\]=\\]`, "Lua long string delimiter", RiskLevelMedium},
		{`loadstring\s*\(`, "Lua loadstring (dynamic code execution)", RiskLevelHigh},
	}

	for _, p := range luaPatterns {
		if matched := regexp.MustCompile(p.pattern).FindString(readme); matched != "" {
			signals = append(signals, MaliciousSignal{
				Type:        "obfuscation_lua",
				Description: p.description,
				Severity:    p.severity,
				Evidence:    truncate(matched, 200),
				Confidence:  0.8,
			})
		}
	}

	// JavaScript obfuscation patterns
	jsPatterns := []struct {
		pattern     string
		description string
		severity    RiskLevel
	}{
		{`eval\s*\(\s*atob\s*\(`, "eval(atob(...)) - base64 decoded eval", RiskLevelCritical},
		{`eval\s*\(\s*["']?\s*base64`, "eval with base64", RiskLevelCritical},
		{`Function\s*\(\s*["']?\s*[A-Za-z0-9+/]{100,}`, "Function constructor with large base64", RiskLevelHigh},
		{`["']([A-Za-z0-9+/]{500,}={0,2})["']`, "Large base64 string literal (>500 chars)", RiskLevelMedium},
		{`\\x[0-9A-Fa-f]{2}`, "Hex escape sequences", RiskLevelMedium},
		{`\\u[0-9A-Fa-f]{4}`, "Unicode escape sequences", RiskLevelMedium},
	}

	for _, p := range jsPatterns {
		if matched := regexp.MustCompile(p.pattern).FindString(readme); matched != "" {
			signals = append(signals, MaliciousSignal{
				Type:        "obfuscation_js",
				Description: p.description,
				Severity:    p.severity,
				Evidence:    truncate(matched, 200),
				Confidence:  0.75,
			})
		}
	}

	// Generic base64 blobs
	base64Regex := regexp.MustCompile(`[A-Za-z0-9+/]{200,}={0,2}`)
	matches := base64Regex.FindAllString(readme, -1)
	if len(matches) > 0 {
		totalLen := 0
		for _, m := range matches {
			totalLen += len(m)
		}
		if totalLen > 1000 {
			signals = append(signals, MaliciousSignal{
				Type:        "obfuscation_base64",
				Description: "Large base64 encoded content detected",
				Severity:    RiskLevelMedium,
				Evidence:    "total_base64_chars=" + strconv.Itoa(totalLen) + ", chunks=" + strconv.Itoa(len(matches)),
				Confidence:  0.7,
			})
		}
	}

	return signals
}

// checkAccountAnomaly checks for suspicious account characteristics.
func (d *MaliciousDetector) checkAccountAnomaly(repoInfo RepositoryInfo) *MaliciousSignal {
	var anomalies []string
	severity := RiskLevelLow

	// Account age check
	if repoInfo.OwnerCreatedAt != nil {
		age := time.Since(*repoInfo.OwnerCreatedAt)
		if age < d.AccountAgeThreshold {
			anomalies = append(anomalies, "account_age="+age.String())
		}
	}

	// Followers check
	if repoInfo.OwnerFollowers != nil && *repoInfo.OwnerFollowers <= d.MinFollowers {
		anomalies = append(anomalies, "followers=0")
	}

	// Profile completeness check (bio, location, etc.)
	if repoInfo.OwnerBio == nil || *repoInfo.OwnerBio == "" {
		anomalies = append(anomalies, "no_bio")
	}

	// Repository count check
	if repoInfo.OwnerRepos != nil && *repoInfo.OwnerRepos < d.MinRepos {
		anomalies = append(anomalies, "repos="+strconv.Itoa(*repoInfo.OwnerRepos))
	}

	// Need at least 2 anomalies to trigger (per spec: new account + 0 followers + no bio + few repos)
	if len(anomalies) < 2 {
		return nil
	}

	// Determine severity based on number of anomalies
	switch len(anomalies) {
	case 2:
		severity = RiskLevelMedium
	case 3:
		severity = RiskLevelHigh
	default: // 4+
		severity = RiskLevelCritical
	}

	return &MaliciousSignal{
		Type:        "account_anomaly",
		Description: "Suspicious account: " + strings.Join(anomalies, ", "),
		Severity:    severity,
		Evidence:    strings.Join(anomalies, "; "),
		Confidence:  0.7,
	}
}

// checkNonTextRatio checks for high ratio of non-printable characters.
func (d *MaliciousDetector) checkNonTextRatio(readme string) *MaliciousSignal {
	if len(readme) == 0 {
		return nil
	}

	nonPrintable := 0
	for _, r := range readme {
		// Printable ASCII range: 32-126, plus common whitespace (tab, newline, carriage return)
		if (r < 32 && r != '\t' && r != '\n' && r != '\r') || r > 126 {
			nonPrintable++
		}
	}

	ratio := float64(nonPrintable) / float64(len(readme))
	if ratio > d.NonTextRatioThreshold {
		return &MaliciousSignal{
			Type:        "non_text_ratio",
			Description: "High ratio of non-printable characters in README",
			Severity:    RiskLevelHigh,
			Evidence:    "ratio=" + formatFloat(ratio, 3) + ", non_printable=" + strconv.Itoa(nonPrintable) + "/" + strconv.Itoa(len(readme)),
			Confidence:  0.8,
		}
	}
	return nil
}
// hasMarkdownStructure checks if content has standard markdown structure.
func (d *MaliciousDetector) hasMarkdownStructure(content string) bool {
	// Check for common markdown patterns (multiline mode for ^ anchor)
	hasHeading := regexp.MustCompile(`(?m)^#{1,6}\s+`).MatchString(content)
	hasParagraph := regexp.MustCompile(`\n\s*\n`).MatchString(content) // blank line separation
	hasList := regexp.MustCompile(`(?m)^[\s]*[-*+]\s+`).MatchString(content)
	hasCodeBlock := regexp.MustCompile("```").MatchString(content)
	hasLink := regexp.MustCompile(`\[.*?\]\(.*?\)`).MatchString(content)

	// At least 2 structural elements
	count := 0
	if hasHeading {
		count++
	}
	if hasParagraph {
		count++
	}
	if hasList {
		count++
	}
	if hasCodeBlock {
		count++
	}
	if hasLink {
		count++
	}

	return count >= 2
}

// aggregateRisk computes overall risk level and confidence from signals.
func (d *MaliciousDetector) aggregateRisk(signals []MaliciousSignal) (RiskLevel, float64) {
	if len(signals) == 0 {
		return RiskLevelLow, 1.0
	}

	// Count by severity
	critical := 0
	high := 0
	medium := 0
	low := 0

	var totalConfidence float64
	for _, s := range signals {
		totalConfidence += s.Confidence
		switch s.Severity {
		case RiskLevelCritical:
			critical++
		case RiskLevelHigh:
			high++
		case RiskLevelMedium:
			medium++
		case RiskLevelLow:
			low++
		}
	}

	avgConfidence := totalConfidence / float64(len(signals))

	// Determine overall risk level
	var riskLevel RiskLevel
	if critical > 0 {
		riskLevel = RiskLevelCritical
	} else if high >= 2 {
		riskLevel = RiskLevelCritical
	} else if high > 0 || medium >= 3 {
		riskLevel = RiskLevelHigh
	} else if medium > 0 || low >= 3 {
		riskLevel = RiskLevelMedium
	} else {
		riskLevel = RiskLevelLow
	}

	return riskLevel, avgConfidence
}

// RepositoryInfo holds repository metadata for account anomaly detection.
type RepositoryInfo struct {
	OwnerCreatedAt *time.Time
	OwnerFollowers *int
	OwnerBio       *string
	OwnerRepos     *int
}

// formatFloat formats a float64 to string with given precision.
func formatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

// truncate truncates a string to max length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// calculateEntropy calculates Shannon entropy of a string (by runes).
func calculateEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	runeCount := 0
	for _, r := range s {
		freq[r]++
		runeCount++
	}

	entropy := 0.0
	length := float64(runeCount)
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}