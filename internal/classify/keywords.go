// Package classify implements Taiwan relevance classification.
package classify

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// KeywordConfig holds all Taiwan keyword classifications (§5.1, §29).
type KeywordConfig struct {
	Government     []string `yaml:"government"`
	GovDomains     []string `yaml:"gov_domains"`
	Finance        []string `yaml:"finance"`
	RealEstate     []string `yaml:"real_estate"`
	Payment        []string `yaml:"payment"`
	Language       []string `yaml:"language"`
	CompanyService []string `yaml:"company_service"`
	DataSources    []string `yaml:"data_sources"`
}

// DomainConfig holds Taiwan official domain configurations (§30).
type DomainConfig struct {
	Domains []struct {
		Domain        string `yaml:"domain"`
		Classification string `yaml:"classification"` // official|government|financial
	} `yaml:"domains"`
}

// KeywordMatchResult holds a matched keyword with its category.
type KeywordMatchResult struct {
	Category string
	Keyword  string
}
//go:embed config/keywords.yaml
var keywordsYAML []byte

//go:embed config/domains.yaml
var domainsYAML []byte

var (
	keywordCfg   KeywordConfig
	domainCfg    DomainConfig
	configLoaded bool
)


// LoadConfig loads keyword and domain configs from embedded YAML files.
func LoadConfig() error {
	if err := yaml.Unmarshal(keywordsYAML, &keywordCfg); err != nil {
		return fmt.Errorf("parse keywords.yaml: %w", err)
	}
	if err := yaml.Unmarshal(domainsYAML, &domainCfg); err != nil {
		return fmt.Errorf("parse domains.yaml: %w", err)
	}
	configLoaded = true
	return nil
}

// allKeywords returns every keyword from all categories.
func (c *KeywordConfig) allKeywords() []string {
	var result []string
	result = append(result, c.Government...)
	result = append(result, c.GovDomains...)
	result = append(result, c.Finance...)
	result = append(result, c.RealEstate...)
	result = append(result, c.Payment...)
	result = append(result, c.Language...)
	result = append(result, c.CompanyService...)
	result = append(result, c.DataSources...)
	return result
}

// MatchTaiwanKeywords searches text fields for Taiwan keywords (§TASK-012).
// Case-insensitive matching across: repo name, owner, description, README, topics, data source URLs.
func MatchTaiwanKeywords(texts []string) []KeywordMatchResult {
	if !configLoaded {
		_ = LoadConfig()
	}

	results := make(map[string]bool)
	var matched []KeywordMatchResult

	for _, kw := range keywordCfg.allKeywords() {
		kwLower := strings.ToLower(kw)
		for _, text := range texts {
			if strings.Contains(strings.ToLower(text), kwLower) {
				key := kwLower
				if !results[key] {
					results[key] = true
					matched = append(matched, KeywordMatchResult{
						Keyword:  kw,
						Category: keywordCategory(kw),
					})
				}
				break
			}
		}
	}

	return matched
}

// keywordCategory returns the category for a keyword.
func keywordCategory(kw string) string {
	for _, c := range keywordCfg.Government {
		if c == kw {
			return "government"
		}
	}
	for _, c := range keywordCfg.GovDomains {
		if c == kw {
			return "government_domain"
		}
	}
	for _, c := range keywordCfg.Finance {
		if c == kw {
			return "finance"
		}
	}
	for _, c := range keywordCfg.RealEstate {
		if c == kw {
			return "real_estate"
		}
	}
	for _, c := range keywordCfg.Payment {
		if c == kw {
			return "payment"
		}
	}
	for _, c := range keywordCfg.Language {
		if c == kw {
			return "language"
		}
	}
	for _, c := range keywordCfg.CompanyService {
		if c == kw {
			return "company_service"
		}
	}
	for _, c := range keywordCfg.DataSources {
		if c == kw {
			return "data_source"
		}
	}
	return "unknown"
}

// MatchOfficialDomains searches URLs for Taiwan official domains (§30).
func MatchOfficialDomains(urls []string) []string {
	if !configLoaded {
		_ = LoadConfig()
	}

	matched := []string{}
	for _, u := range urls {
		urlLower := strings.ToLower(u)
		for _, d := range domainCfg.Domains {
			if strings.Contains(urlLower, d.Domain) {
				matched = append(matched, d.Domain)
				break
			}
		}
	}
	return matched
}

// MatchDataSource scans URLs for Taiwan data source domains.
func MatchDataSource(urls []string) []string {
	if !configLoaded {
		_ = LoadConfig()
	}

	matched := []string{}
	for _, u := range urls {
		urlLower := strings.ToLower(u)
		for _, kw := range keywordCfg.DataSources {
			if strings.Contains(urlLower, strings.ToLower(kw)) {
				matched = append(matched, kw)
				break
			}
		}
	}
	return matched
}
