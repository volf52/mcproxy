package config

import (
	"testing"
)

func TestDiscriminatedUnionBasic(t *testing.T) {
	// Test basic HTTP endpoint creation through JSON unmarshaling
	jsonStr := `{
		"endpoints": {
			"test": {
				"type": "http",
				"url": "https://example.com",
				"headers": {
					"Authorization": "Bearer {{token}}"
				}
			}
		}
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsonStr), &config, "")
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	// Test type assertion
	httpEndpoint, ok := config.Endpoints["test"].Value.(HttpEndpoint)
	if !ok {
		t.Fatalf("Expected HttpEndpoint, got %T", config.Endpoints["test"].Value)
	}

	if httpEndpoint.Type != EndpointTypeHTTP {
		t.Errorf("Expected type 'http', got '%s'", httpEndpoint.Type)
	}

	if httpEndpoint.Url != "https://example.com" {
		t.Errorf("Expected URL 'https://example.com', got '%s'", httpEndpoint.Url)
	}

	if httpEndpoint.Headers["Authorization"] != "Bearer {{token}}" {
		t.Errorf("Expected Authorization header, got '%s'", httpEndpoint.Headers["Authorization"])
	}
}

func TestStdioEndpointDiscriminatedUnion(t *testing.T) {
	// Test basic stdio endpoint creation through JSON unmarshaling
	jsonStr := `{
		"endpoints": {
			"stdio-test": {
				"type": "stdio",
				"command": "/usr/local/bin/mcp-server",
				"env": {
					"API_KEY": "{{token}}"
				}
			}
		}
	}`

	var config Config
	err := UnmarshalWithAutoDetection([]byte(jsonStr), &config, "")
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if len(config.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(config.Endpoints))
	}

	// Test type assertion
	stdioEndpoint, ok := config.Endpoints["stdio-test"].Value.(StdioEndpoint)
	if !ok {
		t.Fatalf("Expected StdioEndpoint, got %T", config.Endpoints["stdio-test"].Value)
	}

	if stdioEndpoint.Type != EndpointTypeStdio {
		t.Errorf("Expected type 'stdio', got '%s'", stdioEndpoint.Type)
	}

	if stdioEndpoint.Command != "/usr/local/bin/mcp-server" {
		t.Errorf("Expected command '/usr/local/bin/mcp-server', got '%s'", stdioEndpoint.Command)
	}

	if stdioEndpoint.Env["API_KEY"] != "{{token}}" {
		t.Errorf("Expected API_KEY env var, got '%s'", stdioEndpoint.Env["API_KEY"])
	}
}
