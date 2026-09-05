package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TaiwanSignals holds configurable Taiwan detection signals.
type TaiwanSignals struct {
	OfficialDomains    []string `yaml:"official_domains"`
	GovernmentAgencies []string `yaml:"government_agencies"`
	FinancialKeywords  []string `yaml:"financial_keywords"`
	DatasetKeywords    []string `yaml:"dataset_keywords"`
	PaymentKeywords    []string `yaml:"payment_keywords"`
	TaiwanKeywords     []string `yaml:"taiwan_keywords"`
	LanguageKeywords   []string `yaml:"language_keywords"`
	CompanyKeywords    []string `yaml:"company_keywords"`
}

// DefaultTaiwanSignals returns built-in default Taiwan signals.
func DefaultTaiwanSignals() *TaiwanSignals {
	return &TaiwanSignals{
		OfficialDomains: []string{
			"twse.com.tw", "tpex.org.tw", "taifex.com.tw",
			"cwa.gov.tw", "moi.gov.tw", "moea.gov.tw",
			"moj.gov.tw", "ly.gov.tw", "judicial.gov.tw",
			"data.gov.tw", "gov.tw",
		},
		GovernmentAgencies: []string{
			"CWA", "MOI", "MOEA", "MOL", "MOF", "PCC", "LY",
			"Judicial Yuan", "data.gov.tw", "gov.tw",
		},
		FinancialKeywords: []string{
			"TWSE", "TPEx", "TAIFEX", "TDCC", "FinMind", "Fugle",
			"台股", "上市", "上櫃",
		},
		DatasetKeywords: []string{
			"實價登錄", "LVR", "land.moi.gov.tw",
			"房價", "房地產", "土地", "預售屋",
		},
		PaymentKeywords: []string{
			"ECPay", "NewebPay", "綠界", "藍新",
		},
		TaiwanKeywords: []string{
			"Taiwan", "Taiwanese", "台灣", "臺灣", "TW", "Taipei",
		},
		LanguageKeywords: []string{
			"zh-TW", "繁體中文", "繁體", "Traditional Chinese",
			"Taiwan Mandarin", "注音", "TOCFL",
		},
		CompanyKeywords: []string{
			"SHOPLINE",
		},
	}
}

// LoadTaiwanSignals loads Taiwan signals from YAML file.
// Returns default signals if file doesn't exist or is empty.
func LoadTaiwanSignals(path string) (*TaiwanSignals, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultTaiwanSignals(), nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return DefaultTaiwanSignals(), nil
	}

	var signals TaiwanSignals
	if err := yaml.Unmarshal(data, &signals); err != nil {
		return nil, err
	}

	// Merge with defaults for any missing fields
	defaults := DefaultTaiwanSignals()
	if len(signals.OfficialDomains) == 0 {
		signals.OfficialDomains = defaults.OfficialDomains
	}
	if len(signals.GovernmentAgencies) == 0 {
		signals.GovernmentAgencies = defaults.GovernmentAgencies
	}
	if len(signals.FinancialKeywords) == 0 {
		signals.FinancialKeywords = defaults.FinancialKeywords
	}
	if len(signals.DatasetKeywords) == 0 {
		signals.DatasetKeywords = defaults.DatasetKeywords
	}
	if len(signals.PaymentKeywords) == 0 {
		signals.PaymentKeywords = defaults.PaymentKeywords
	}
	if len(signals.TaiwanKeywords) == 0 {
		signals.TaiwanKeywords = defaults.TaiwanKeywords
	}
	if len(signals.LanguageKeywords) == 0 {
		signals.LanguageKeywords = defaults.LanguageKeywords
	}
	if len(signals.CompanyKeywords) == 0 {
		signals.CompanyKeywords = defaults.CompanyKeywords
	}

	return &signals, nil
}

// AISignals holds configurable AI detection signals.
type AISignals struct {
	CoreAIKeywords      []string `yaml:"core_ai_keywords"`
	AgentKeywords       []string `yaml:"agent_keywords"`
	RAGKeywords         []string `yaml:"rag_keywords"`
	LLMProviderKeywords []string `yaml:"llm_provider_keywords"`
	MCPKeywords         []string `yaml:"mcp_keywords"`
	FrameworkKeywords   []string `yaml:"framework_keywords"`
	PackagePatterns     []string `yaml:"package_patterns"`
}

// DefaultAISignals returns built-in default AI signals.
func DefaultAISignals() *AISignals {
	return &AISignals{
		CoreAIKeywords: []string{
			"AI", "Artificial Intelligence", "LLM", "Large Language Model",
			"Generative AI", "GenAI", "Machine Learning", "Deep Learning",
		},
		AgentKeywords: []string{
			"agent", "agentic", "AI agent", "AI assistant", "autonomous agent",
		},
		RAGKeywords: []string{
			"RAG", "retrieval", "embedding", "vector", "vector database", "semantic search",
		},
		LLMProviderKeywords: []string{
			"Claude", "ChatGPT", "Gemini", "OpenAI", "Anthropic", "Cohere", "Mistral", "Llama",
		},
		MCPKeywords: []string{
			"MCP", "Model Context Protocol", "tool calling", "function calling", "AI workflow",
		},
		FrameworkKeywords: []string{
			"LangChain", "LlamaIndex", "AutoGen", "CrewAI", "Semantic Kernel", "LangGraph",
		},
		PackagePatterns: []string{
			"@modelcontextprotocol/", "langchain", "llamaindex", "openai",
			"anthropic", "google-generativeai",
		},
	}
}

// LoadAISignals loads AI signals from YAML file.
// Returns default signals if file doesn't exist or is empty.
func LoadAISignals(path string) (*AISignals, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultAISignals(), nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return DefaultAISignals(), nil
	}

	var signals AISignals
	if err := yaml.Unmarshal(data, &signals); err != nil {
		return nil, err
	}

	// Merge with defaults for any missing fields
	defaults := DefaultAISignals()
	if len(signals.CoreAIKeywords) == 0 {
		signals.CoreAIKeywords = defaults.CoreAIKeywords
	}
	if len(signals.AgentKeywords) == 0 {
		signals.AgentKeywords = defaults.AgentKeywords
	}
	if len(signals.RAGKeywords) == 0 {
		signals.RAGKeywords = defaults.RAGKeywords
	}
	if len(signals.LLMProviderKeywords) == 0 {
		signals.LLMProviderKeywords = defaults.LLMProviderKeywords
	}
	if len(signals.MCPKeywords) == 0 {
		signals.MCPKeywords = defaults.MCPKeywords
	}
	if len(signals.FrameworkKeywords) == 0 {
		signals.FrameworkKeywords = defaults.FrameworkKeywords
	}
	if len(signals.PackagePatterns) == 0 {
		signals.PackagePatterns = defaults.PackagePatterns
	}

	return &signals, nil
}