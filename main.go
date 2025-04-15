package main

import (
	"log"

	"github.com/mark3labs/mcp-go/server"
)

func main() {
	parser := NewParser()
	err := parser.ParseFileV2("doc.json")
	if err != nil {
		log.Fatalf("Failed to parse OpenAPI document: %v", err)
	}
	converter := NewConverter(parser, ConvertOptions{})
	s, err := converter.Convert()
	if err != nil {
		log.Fatalf("Failed to convert OpenAPI to MCP: %v", err)
	}
	server.NewSSEServer(s).Start("127.0.0.1:3001")
}
