package config

import (
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

type HttpEndpoint struct {
	EndpointShared
	Type    EndpointType      `json:"type" enum:"http" description:"Endpoint type: 'http' for HTTP upstream or 'stdio' for MCP stdio process (default: 'http')"`
	Url     string            `json:"url" description:"The upstream server url"`
	Headers map[string]string `json:"headers" description:"Custom headers to add to requests to the upstream server."`
}

func (e HttpEndpoint) isContainer() {}

type StdioEndpoint struct {
	EndpointShared
	Type    EndpointType      `json:"type" enum:"stdio" description:"Endpoint type: 'http' for HTTP upstream or 'stdio' for MCP stdio process (default: 'http')"`
	Command string            `json:"command" description:"Command to execute for stdio endpoints"`
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

	if err := UnmarshalWithAutoDetection(data, &discriminator, ""); err != nil {
		return err
	}

	switch discriminator.Type {
	case EndpointTypeHTTP:
		var httpEndpoint HttpEndpoint
		if err := UnmarshalWithAutoDetection(data, &httpEndpoint, ""); err != nil {
			return err
		}
		e.Value = httpEndpoint
	case EndpointTypeStdio:
		var stdioEndpoint StdioEndpoint
		if err := UnmarshalWithAutoDetection(data, &stdioEndpoint, ""); err != nil {
			return err
		}
		e.Value = stdioEndpoint
	default:
		return fmt.Errorf("Unknown endpoint type: %s", discriminator.Type)
	}

	return nil
}

func AddGeneratorReflection(ref *jsonschema.Reflector) error {

	return nil
}
