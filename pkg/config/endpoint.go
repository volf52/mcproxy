package config

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/swaggest/jsonschema-go"
)

type EndpointInner interface {
	isContainer()
}

// EndpointType represents the type of endpoint (HTTP or stdio)
type EndpointType string

const (
	EndpointTypeHTTP  EndpointType = "http"
	EndpointTypeStdio EndpointType = "stdio"
)

type EndpointShared struct {
	MaxBodySize *int64         `json:"maxBodySize,omitempty" description:"Maximum request body size in bytes (overrides global limit)."`
	Timeout     *time.Duration `json:"timeout,omitempty" description:"Per-endpoint timeout in seconds (overrides global timeout)."`
}

func (e *EndpointShared) UnmarshalJSON(data []byte) error {
	type Alias EndpointShared
	aux := &struct {
		Timeout interface{} `json:"timeout,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle timeout parsing
	if aux.Timeout != nil {
		switch v := aux.Timeout.(type) {
		case string:
			// Parse duration string like "30s"
			duration, err := time.ParseDuration(v)
			if err != nil {
				return fmt.Errorf("invalid timeout duration: %v", err)
			}
			e.Timeout = &duration
		case float64:
			// Handle numeric value (interpreted as seconds)
			duration := time.Duration(v * float64(time.Second))
			e.Timeout = &duration
		case int64:
			// Handle integer value (interpreted as seconds)
			duration := time.Duration(v * int64(time.Second))
			e.Timeout = &duration
		default:
			return fmt.Errorf("unsupported timeout type: %T", aux.Timeout)
		}
	}

	return nil
}

type HttpEndpoint struct {
	EndpointShared
	Type    EndpointType      `json:"type" enum:"http" description:"Endpoint type: 'http' for HTTP upstream or 'stdio' for MCP stdio process (default: 'http')" required:"true"`
	Url     string            `json:"url" description:"The upstream server url" required:"true"`
	Headers map[string]string `json:"headers" description:"Custom headers to add to requests to the upstream server."`
}

func (e HttpEndpoint) isContainer() {}

type StdioEndpoint struct {
	EndpointShared
	Type    EndpointType      `json:"type" enum:"stdio" description:"Endpoint type: 'http' for HTTP upstream or 'stdio' for MCP stdio process (default: 'http')" required:"true"`
	Command string            `json:"command" description:"Command to execute for stdio endpoints" required:"true"`
	Args    []string          `json:"args,omitempty" description:"Arguments to pass to the MCP stdio endpoint during initialization"`
	Env     map[string]string `json:"env,omitempty" description:"Environment variables to set for the stdio process"`
}

func (e StdioEndpoint) isContainer() {}

type Endpoint struct {
	Value EndpointInner
}

func (e *Endpoint) UnmarshalJSON(data []byte) error {
	var discriminator struct {
		Type EndpointType `json:"type"`
	}

	// First, unmarshal the discriminator to determine the type
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return err
	}

	// Default to HTTP if type is not specified
	if discriminator.Type == "" {
		discriminator.Type = EndpointTypeHTTP
	}

	// Use a temporary struct to unmarshal the data
	var temp struct {
		Type        EndpointType      `json:"type"`
		Url         string            `json:"url,omitempty"`
		Headers     map[string]string `json:"headers,omitempty"`
		Command     string            `json:"command,omitempty"`
		Args        []string          `json:"args,omitempty"`
		Env         map[string]string `json:"env,omitempty"`
		Timeout     interface{}       `json:"timeout,omitempty"`
		MaxBodySize interface{}       `json:"maxBodySize,omitempty"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	// Create the appropriate endpoint type
	switch discriminator.Type {
	case EndpointTypeHTTP:
		var httpEndpoint HttpEndpoint
		httpEndpoint.Type = temp.Type
		httpEndpoint.Url = temp.Url
		httpEndpoint.Headers = temp.Headers
		// Handle shared fields
		if temp.Timeout != nil {
			switch v := temp.Timeout.(type) {
			case string:
				duration, err := time.ParseDuration(v)
				if err != nil {
					return fmt.Errorf("invalid timeout duration: %v", err)
				}
				httpEndpoint.Timeout = &duration
			case float64:
				duration := time.Duration(v * float64(time.Second))
				httpEndpoint.Timeout = &duration
			case int64:
				duration := time.Duration(v * int64(time.Second))
				httpEndpoint.Timeout = &duration
			default:
				return fmt.Errorf("unsupported timeout type: %T", temp.Timeout)
			}
		}
		if temp.MaxBodySize != nil {
			switch v := temp.MaxBodySize.(type) {
			case float64:
				maxBodySizeInt := int64(v)
				httpEndpoint.MaxBodySize = &maxBodySizeInt
			case int64:
				httpEndpoint.MaxBodySize = &v
			default:
				return fmt.Errorf("unsupported maxBodySize type: %T", temp.MaxBodySize)
			}
		}
		e.Value = httpEndpoint
	case EndpointTypeStdio:
		var stdioEndpoint StdioEndpoint
		stdioEndpoint.Type = temp.Type
		stdioEndpoint.Command = temp.Command
		stdioEndpoint.Args = temp.Args
		stdioEndpoint.Env = temp.Env
		// Handle shared fields
		if temp.Timeout != nil {
			switch v := temp.Timeout.(type) {
			case string:
				duration, err := time.ParseDuration(v)
				if err != nil {
					return fmt.Errorf("invalid timeout duration: %v", err)
				}
				stdioEndpoint.Timeout = &duration
			case float64:
				duration := time.Duration(v * float64(time.Second))
				stdioEndpoint.Timeout = &duration
			case int64:
				duration := time.Duration(v * int64(time.Second))
				stdioEndpoint.Timeout = &duration
			default:
				return fmt.Errorf("unsupported timeout type: %T", temp.Timeout)
			}
		}
		if temp.MaxBodySize != nil {
			switch v := temp.MaxBodySize.(type) {
			case float64:
				maxBodySizeInt := int64(v)
				stdioEndpoint.MaxBodySize = &maxBodySizeInt
			case int64:
				stdioEndpoint.MaxBodySize = &v
			default:
				return fmt.Errorf("unsupported maxBodySize type: %T", temp.MaxBodySize)
			}
		}
		e.Value = stdioEndpoint
	default:
		return fmt.Errorf("Unknown endpoint type: %s", discriminator.Type)
	}

	return nil
}

func AddGeneratorReflection(ref *jsonschema.Reflector) error {
	// Reflect types and ensure they have titles to encourage named definitions
	httpSchema, err := ref.Reflect(HttpEndpoint{})
	if err != nil {
		return err
	}
	httpSchema.WithTitle("HTTP Endpoint")
	ref.AddTypeMapping(HttpEndpoint{}, httpSchema)

	stdioSchema, err := ref.Reflect(StdioEndpoint{})
	if err != nil {
		return err
	}
	stdioSchema.WithTitle("Stdio Endpoint")
	ref.AddTypeMapping(StdioEndpoint{}, stdioSchema)

	unionSchema := jsonschema.Schema{}
	unionSchema.WithOneOf(
		httpSchema.ToSchemaOrBool(),
		stdioSchema.ToSchemaOrBool(),
	)

	// Add discriminator for OpenAPI compatibility and better tool support
	discriminator := map[string]interface{}{
		"propertyName": "type",
	}
	unionSchema.ExtraProperties = map[string]interface{}{
		"discriminator": discriminator,
	}

	ref.AddTypeMapping(Endpoint{}, unionSchema)

	return nil
}
