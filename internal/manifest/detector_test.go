package manifest

import (
	"testing"
)

func TestParseJSONManifest_packageJSON(t *testing.T) {
	content := `{"name": "test-mcp", "mcp": {"servers": {"test": {"command": "python", "transport": "stdio"}}}}`
	info, err := ParseManifest(content, "package.json")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.ServerName != "test" {
		t.Errorf("Expected server_name=test, got %s", info.ServerName)
	}
	if info.Transport != "stdio" {
		t.Errorf("Expected transport=stdio, got %s", info.Transport)
	}
	if info.Command != "python" {
		t.Errorf("Expected command=python, got %s", info.Command)
	}
}

func TestParseJSONManifest_mcpJSON(t *testing.T) {
	content := `{"mcpServers": {"my-server": {"command": "node", "args": ["server.js"], "env": {"API_KEY": "123"}}}}`
	info, err := ParseManifest(content, "mcp.json")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.ServerName != "my-server" {
		t.Errorf("Expected server_name=my-server, got %s", info.ServerName)
	}
	if info.Command != "node" {
		t.Errorf("Expected command=node, got %s", info.Command)
	}
	if len(info.Args) != 1 || info.Args[0] != "server.js" {
		t.Errorf("Expected args=[server.js], got %v", info.Args)
	}
	if v, ok := info.Env["API_KEY"]; !ok || v != "123" {
		t.Errorf("Expected env API_KEY=123, got %v", info.Env["API_KEY"])
	}
}

func TestParsePyprojectTOML(t *testing.T) {
	content := `[project.entry-points."mcp"]
"my-server" = "mcp_server:main"
command = "python"
transport = "stdio"
`
	info, err := ParseManifest(content, "pyproject.toml")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.Transport != "stdio" {
		t.Errorf("Expected transport=stdio, got %s", info.Transport)
	}
}

func TestParseGoMod(t *testing.T) {
	content := `module github.com/foo/bar-mcp
go 1.22
`
	info, err := ParseManifest(content, "go.mod")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.ServerName != "github.com/foo/bar-mcp" {
		t.Errorf("Expected server_name=github.com/foo/bar-mcp, got %s", info.ServerName)
	}
	if info.Transport != "stdio" {
		t.Errorf("Expected transport=stdio, got %s", info.Transport)
	}
}

func TestParseCargoToml(t *testing.T) {
	content := `[package]
name = "mcp-server"

[package.metadata.mcp]
command = "cargo"
transport = "stdio"
`
	info, err := ParseManifest(content, "Cargo.toml")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if info.ServerName != "mcp-server" {
		t.Errorf("Expected server_name=mcp-server, got %s", info.ServerName)
	}
	if info.Command != "cargo" {
		t.Errorf("Expected command=cargo, got %s", info.Command)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := ParseManifest("{invalid json}", "server.json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParseUnsupportedType(t *testing.T) {
	_, err := ParseManifest("content", "unknown.toml")
	if err == nil {
		t.Error("Expected error for unsupported type")
	}
}
