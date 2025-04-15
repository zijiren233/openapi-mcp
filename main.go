package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	parser := NewParser()
	err := parser.ParseFile("doc.json")
	if err != nil {
		log.Fatalf("Failed to parse OpenAPI document: %v", err)
	}
	converter := NewConverter(parser, ConvertOptions{
		ServerName: "openapi-server",
		Version:    "1.0.0",
	})
	s, err := converter.Convert()
	if err != nil {
		log.Fatalf("Failed to convert OpenAPI to MCP: %v", err)
	}
	server.ServeStdio(s)
}
