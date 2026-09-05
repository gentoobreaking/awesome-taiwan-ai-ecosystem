package security

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/david/awesome-taiwan-mcp/internal/models"
)

const (
	// MaliciousType is the finding type for malicious repository detection.
	MaliciousType = "malicious_repository"
)

// MaliciousSignal represents a single malicious indicator.
type MaliciousSignal struct {
	Name        string  // e.g., "high_entropy", "lua_bytecode", "throwaway_account"
	Description string  // human-readable
	Confidence  float64 // 0.0-1.0
	Evidence    string  // matched snippet or metric value
}

// MaliciousResult holds the result of malicious detection.
type MaliciousResult struct {
	RiskLevel   string            // LOW, MEDIUM, HIGH, CRITICAL
	Score       float64           // 0.0-100.0
	Signals     []MaliciousSignal // detected signals
	Recommend   string            // "monitor", "investigate", "block", "report"
	ScannedAt   time.Time
}

// MaliciousDetectorConfig holds configuration for malicious detection.
type MaliciousDetectorConfig struct {
	EntropyThreshold        float64       // Shannon entropy > this = suspicious (default 7.0)
	MaxReadmeSize           int           // README > this bytes = suspicious (default 100KB)
	NonPrintableThreshold   float64       // non-printable ratio > this = suspicious (default 0.30)
	AccountAgeThreshold     time.Duration // account age < this = suspicious (default 90 days)
	MinFollowers            int           // followers < this = suspicious (default 0)
	MinProfileFields        int           // profile fields < this = suspicious (default 1)
	MaxReposForNewAccount   int           // repos > this for new account = suspicious (default 5)
	EnableObfuscationDetect bool          // enable obfuscation pattern matching
}

// DefaultMaliciousDetectorConfig returns sensible defaults.
func DefaultMaliciousDetectorConfig() MaliciousDetectorConfig {
	return MaliciousDetectorConfig{
		EntropyThreshold:        7.0,
		MaxReadmeSize:           100 * 1024, // 100 KB
		NonPrintableThreshold:   0.30,
		AccountAgeThreshold:     90 * 24 * time.Hour,
		MinFollowers:            0,
		MinProfileFields:        1,
		MaxReposForNewAccount:   5,
		EnableObfuscationDetect: true,
	}
}

// MaliciousDetector detects malicious repository patterns.
type MaliciousDetector struct {
	config MaliciousDetectorConfig
	// Compiled regex patterns for obfuscation detection
	obfuscationPatterns []*regexp.Regexp
}

// NewMaliciousDetector creates a new malicious detector with default config.
func NewMaliciousDetector() *MaliciousDetector {
	return NewMaliciousDetectorWithConfig(DefaultMaliciousDetectorConfig())
}

// NewMaliciousDetectorWithConfig creates a new malicious detector with custom config.
func NewMaliciousDetectorWithConfig(config MaliciousDetectorConfig) *MaliciousDetector {
	d := &MaliciousDetector{
		config: config,
	}
	d.compileObfuscationPatterns()
	return d
}

// compileObfuscationPatterns compiles regex patterns for obfuscation detection.
func (d *MaliciousDetector) compileObfuscationPatterns() {
	patterns := []string{
		// Lua VM bytecode patterns (e.g., clearsdunker-create/ez)
		`while\s+W\[\d+\]\s+do`,
		`W\[0x[0-9A-Fa-f]+\]`,
		`W\.\w+\(\(W\.\w+\(`,
		`return\s+nil;\s*end;\s*return\s+nil`,
		`0x[0-9A-Fa-f]{2,}__`,

		// JavaScript obfuscation
		`eval\s*\(\s*atob\s*\(`,
		`eval\s*\(\s*decodeURIComponent\s*\(`,
		`String\.fromCharCode\s*\([^)]{50,}`,
		`\\x[0-9a-f]{2}\\x[0-9a-f]{2}\\x[0-9a-f]{2}`,

		// Base64 blobs (long continuous base64)
		`[A-Za-z0-9+/]{200,}={0,2}`,

		// Python obfuscation
		`exec\s*\(\s*base64\.b64decode\s*\(`,
		`__import__\s*\(\s*['\"]base64['\"]\s*\)`,
	}
	d.obfuscationPatterns = make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		d.obfuscationPatterns[i] = regexp.MustCompile(`(?i)` + p)
	}
}

// Detect runs malicious detection on a server's README and metadata.
func (d *MaliciousDetector) Detect(server *models.MCPServer, readme string, accountCreatedAt *time.Time, followerCount int, profileFieldCount int, repoCount int) MaliciousResult {
	var signals []MaliciousSignal

	// 1. README entropy check
	if d.config.EntropyThreshold > 0 {
		entropy := shannonEntropy(readme)
		if entropy > d.config.EntropyThreshold {
			signals = append(signals, MaliciousSignal{
				Name:        "high_entropy",
				Description: "README has unusually high Shannon entropy (likely binary/obfuscated)",
				Confidence:  math.Min((entropy-d.config.EntropyThreshold)/2.0, 1.0),
				Evidence:    "entropy=" + entropyString(entropy),
			})
		}
	}

	// 2. README size check
	if d.config.MaxReadmeSize > 0 && len(readme) > d.config.MaxReadmeSize {
		signals = append(signals, MaliciousSignal{
			Name:        "oversized_readme",
			Description: "README exceeds typical documentation size",
			Confidence:  math.Min(float64(len(readme))/float64(d.config.MaxReadmeSize*5), 1.0),
			Evidence:    "size=" + sizeString(len(readme)),
		})
	}

	// 3. Non-printable character ratio
	if d.config.NonPrintableThreshold > 0 && len(readme) > 0 {
		nonPrintable := countNonPrintable(readme)
		ratio := float64(nonPrintable) / float64(len(readme))
		if ratio > d.config.NonPrintableThreshold {
			signals = append(signals, MaliciousSignal{
				Name:        "high_nonprintable",
				Description: "README contains high ratio of non-printable characters",
				Confidence:  math.Min(ratio/d.config.NonPrintableThreshold, 1.0),
				Evidence:    "ratio=" + ratioString(ratio),
			})
		}
	}

	// 4. Obfuscation pattern matching
	if d.config.EnableObfuscationDetect {
		for _, pattern := range d.obfuscationPatterns {
			matches := pattern.FindAllString(readme, 5)
			if len(matches) > 0 {
				signals = append(signals, MaliciousSignal{
					Name:        "obfuscation_pattern",
					Description: "Detected code obfuscation pattern: " + pattern.String(),
					Confidence:  0.85,
					Evidence:    "matches: " + strings.Join(matches, ", "),
				})
			}
		}
	}

	// 5. Account anomaly detection (if metadata available)
	if accountCreatedAt != nil && followerCount >= 0 && profileFieldCount >= 0 && repoCount >= 0 {
		accountAge := time.Since(*accountCreatedAt)
		if accountAge < d.config.AccountAgeThreshold {
			isNewAccount := true
			// Check multiple anomaly indicators
			anomalyCount := 0
			if followerCount <= d.config.MinFollowers {
				anomalyCount++
			}
			if profileFieldCount <= d.config.MinProfileFields {
				anomalyCount++
			}
			if repoCount > d.config.MaxReposForNewAccount {
				anomalyCount++
			}
			if isNewAccount && anomalyCount >= 2 {
				signals = append(signals, MaliciousSignal{
					Name:        "throwaway_account",
					Description: "New account with multiple anomaly indicators (low followers, empty profile, many repos)",
					Confidence:  0.75 + 0.1*float64(anomalyCount),
					Evidence:    "age=" + accountAge.Truncate(time.Hour).String() + ", followers=" + intString(followerCount) + ", profile_fields=" + intString(profileFieldCount) + ", repos=" + intString(repoCount),
				})
			}
		}
	}

	// Calculate overall risk
	return d.calculateRisk(signals)
}

// calculateRisk determines overall risk level from signals.
func (d *MaliciousDetector) calculateRisk(signals []MaliciousSignal) MaliciousResult {
	if len(signals) == 0 {
		return MaliciousResult{
			RiskLevel: "LOW",
			Score:     0,
			Signals:   nil,
			Recommend: "monitor",
			ScannedAt: time.Now().UTC(),
		}
	}

	// Weighted score
	var totalScore float64
	for _, s := range signals {
		// Base weight by signal type
		weight := 1.0
		switch s.Name {
		case "lua_bytecode", "obfuscation_pattern":
			weight = 3.0
		case "high_entropy":
			weight = 2.0
		case "throwaway_account":
			weight = 2.5
		case "oversized_readme", "high_nonprintable":
			weight = 1.5
		}
		totalScore += s.Confidence * weight * 10 // scale to 0-100
	}

	// Normalize to 0-100
	score := math.Min(totalScore/float64(len(signals))*5, 100)

	var riskLevel string
	var recommend string
	switch {
	case score >= 75:
		riskLevel = "CRITICAL"
		recommend = "block,report"
	case score >= 50:
		riskLevel = "HIGH"
		recommend = "block,investigate"
	case score >= 25:
		riskLevel = "MEDIUM"
		recommend = "investigate"
	default:
		riskLevel = "LOW"
		recommend = "monitor"
	}

	return MaliciousResult{
		RiskLevel: riskLevel,
		Score:     score,
		Signals:   signals,
		Recommend: recommend,
		ScannedAt: time.Now().UTC(),
	}
}

// --- Helper functions ---


// IsMalicious returns true if risk level is HIGH or CRITICAL.
func (r MaliciousResult) IsMalicious() bool {
	return r.RiskLevel == "HIGH" || r.RiskLevel == "CRITICAL"
}

// ToSecurityFinding converts malicious result to SecurityFinding.
func (r MaliciousResult) ToSecurityFinding(repoURL string) models.SecurityFinding {
	if len(r.Signals) == 0 {
		return models.SecurityFinding{}
	}
	var evidenceParts []string
	for _, s := range r.Signals {
		evidenceParts = append(evidenceParts, s.Name+": "+s.Evidence)
	}
	return models.SecurityFinding{
		Type:     MaliciousType,
		Severity: models.SecuritySeverity(r.RiskLevel),
		Source:   "malicious_detector",
		Location: repoURL,
		Evidence: strings.Join(evidenceParts, "; "),
	}
}
// --- Helper functions ---

func shannonEntropy(data string) float64 {
	if len(data) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, r := range data {
		freq[r]++
	}
	var entropy float64
	length := float64(len(data))
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}


func float64String(f float64, prec int) string {
	return fmt.Sprintf("%."+intString(prec)+"f", f)
}

func intString(i int) string {
	return fmt.Sprintf("%d", i)
}

func int64String(i int64) string {
	return fmt.Sprintf("%d", i)
}

func sizeString(bytes int) string {
	if bytes < 1024 {
		return intString(bytes) + "B"
	}
	if bytes < 1024*1024 {
		return intString(bytes/1024) + "KB"
	}
	return intString(bytes/(1024*1024)) + "MB"
}
func entropyString(e float64) string {
	return float64String(e, 2)
}

func ratioString(r float64) string {
	return float64String(r, 3)
}
func countNonPrintable(s string) int {
	count := 0
	for _, r := range s {
		if r < 32 || r > 126 {
			if r != '\n' && r != '\r' && r != '\t' {
				count++
			}
		}
	}
	return count
}
