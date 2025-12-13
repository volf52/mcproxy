package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"mcproxy/pkg/config"

	jsonschema "github.com/swaggest/jsonschema-go"
)

func main() {
	// Create a reflector
	reflector := jsonschema.Reflector{}

	if e := config.AddGeneratorReflection(&reflector); e != nil {
		log.Fatal(e)
	}

	// type Config struct {
	// 	Foo []config.Endpoint `json:"foo"`
	// }
	// s, e := reflector.Reflect(Config{})
	// if e != nil {
	// 	log.Fatal(e)
	// }
	// jsonBytes, _ := json.MarshalIndent(s, "", "  ")
	// fmt.Println(string(jsonBytes))

	// Generate schema from Config struct
	schema, err := reflector.Reflect(config.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating schema: %v\n", err)
		os.Exit(1)
	}

	// Add basic metadata manually
	if schema.Title == nil {
		schema.WithTitle("MCProxy Configuration Schema")
	}
	if schema.Description == nil {
		schema.WithDescription("Configuration schema for mcproxy HTTP proxy service")
	}
	if schema.ID == nil {
		schema.WithID("https://mcproxy.dev/schemas/config.json")
	}

	// Marshal to JSON
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling schema: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	outputPath := "config.schema.json"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	err = os.WriteFile(outputPath, schemaBytes, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing schema file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("JSON Schema generated successfully: %s\n", outputPath)
}
