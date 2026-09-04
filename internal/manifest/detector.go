// Package manifest provides static parsing of MCP server manifest files.
// No code execution — only static parsing (§TASK-009).
package manifest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ManifestInfo holds parsed manifest data in a unified structure.
type ManifestInfo struct {
	ServerName   string         `json:"server_name"`
	Transport    string         `json:"transport"`
	Command      string         `json:"command"`
	Args         []string       `json:"args"`
	Env          map[string]any `json:"env"`
	Tools        []Tool         `json:"tools"`
	Resources    []Resource     `json:"resources"`
	Prompts      []Prompt       `json:"prompts"`
	MCPVersion   string         `json:"mcp_version"`
	License      string         `json:"license"`
}

// Tool represents an MCP tool definition in a manifest.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// Resource represents an MCP resource in a manifest.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MIMEType    string `json:"mime_type"`
}

// Prompt represents an MCP prompt in a manifest.
type Prompt struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ParseManifest parses manifest content based on file type (§TASK-009).
// Supported types: package.json, pyproject.toml, go.mod, Cargo.toml,
// server.json, mcp.json, manifest.json
func ParseManifest(content string, fileType string) (*ManifestInfo, error) {
	switch strings.ToLower(fileType) {
	case "package.json", "server.json", "mcp.json", "manifest.json":
		return parseJSONManifest(content)
	case "pyproject.toml":
		return parsePyprojectTOML(content)
	case "go.mod":
		return parseGoMod(content)
	case "cargo.toml":
		return parseCargoToml(content)
	default:
		return nil, fmt.Errorf("unsupported manifest type: %s", fileType)
	}
}

func parseJSONManifest(content string) (*ManifestInfo, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	info := &ManifestInfo{
		Env: make(map[string]any),
	}

	// server.json / mcp.json / manifest.json format
	if servers, ok := raw["mcpServers"].(map[string]any); ok {
		for name, cfg := range servers {
			if m, ok := cfg.(map[string]any); ok {
				info.ServerName = name
				if v, ok := m["command"].(string); ok {
					info.Command = v
				}
				if v, ok := m["transport"].(string); ok {
					info.Transport = v
				}
				if args, ok := m["args"].([]any); ok {
					for _, a := range args {
						if s, ok := a.(string); ok {
							info.Args = append(info.Args, s)
						}
					}
				}
				if env, ok := m["env"].(map[string]any); ok {
					for k, v := range env {
						info.Env[k] = v
					}
				}
			}
		}
		return info, nil
	}

	// package.json mcp.servers format
	if pkg, ok := raw["mcp"].(map[string]any); ok {
		if servers, ok := pkg["servers"].(map[string]any); ok {
			for name, cfg := range servers {
				if m, ok := cfg.(map[string]any); ok {
					info.ServerName = name
					if v, ok := m["command"].(string); ok {
						info.Command = v
					}
					if v, ok := m["transport"].(string); ok {
						info.Transport = v
					}
					if v, ok := m["type"].(string); ok {
						info.Transport = v
					}
				}
			}
		}
	}

	// Generic server config
	if v, ok := raw["name"].(string); ok && info.ServerName == "" {
		info.ServerName = v
	}
	if v, ok := raw["transport"].(string); ok {
		info.Transport = v
	}
	if v, ok := raw["command"].(string); ok {
		info.Command = v
	}

	return info, nil
}

func parsePyprojectTOML(content string) (*ManifestInfo, error) {
	info := &ManifestInfo{Env: make(map[string]any)}

	// Find entry-points."mcp" or [tool.mcp] section
	mcpEntries := extractTomlTable(content, `[project.entry-points."mcp"]`)
	if len(mcpEntries) == 0 {
		mcpEntries = extractTomlTable(content, `[tool.mcp]`)
	}

	for _, line := range mcpEntries {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "command") {
			info.Command = extractTomlValue(line)
		} else if strings.HasPrefix(line, "transport") {
			info.Transport = extractTomlValue(line)
		}
	}

	return info, nil
}

func parseGoMod(content string) (*ManifestInfo, error) {
	info := &ManifestInfo{Env: make(map[string]any), Transport: "stdio"}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				info.ServerName = parts[1]
			}
		}
	}
	return info, nil
}

func parseCargoToml(content string) (*ManifestInfo, error) {
	info := &ManifestInfo{Env: make(map[string]any)}
	mcpEntries := extractTomlTable(content, `[package.metadata.mcp]`)
	for _, line := range mcpEntries {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "command") {
			info.Command = extractTomlValue(line)
		} else if strings.HasPrefix(line, "transport") {
			info.Transport = extractTomlValue(line)
		}
	}
	if info.ServerName == "" {
		pkgEntries := extractTomlTable(content, `[package]`)
		for _, line := range pkgEntries {
			if strings.HasPrefix(strings.TrimSpace(line), "name") {
				info.ServerName = extractTomlValue(line)
			}
		}
	}
	return info, nil
}

func extractTomlTable(content, tableName string) []string {
	lines := strings.Split(content, "\n")
	var result []string
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, tableName) {
			inTable = true
			continue
		}
		if inTable && strings.HasPrefix(trimmed, "[") {
			break
		}
		if inTable {
			result = append(result, line)
		}
	}
	return result
}

func extractTomlValue(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) < 2 {
		return ""
	}
	v := strings.TrimSpace(parts[1])
	v = strings.Trim(v, `"`)
	return v
}
