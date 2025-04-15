package main

import (
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ConvertOptions struct {
	ServerName     string
	Version        string
	ToolNamePrefix string
	TemplatePath   string
}

// Converter represents an OpenAPI to MCP converter
type Converter struct {
	parser  *Parser
	options ConvertOptions
}

// NewConverter creates a new OpenAPI to MCP converter
func NewConverter(parser *Parser, options ConvertOptions) *Converter {
	// Set default values if not provided
	if options.ServerName == "" {
		options.ServerName = "openapi-server"
	}

	return &Converter{
		parser:  parser,
		options: options,
	}
}

// Convert converts an OpenAPI document to an MCP configuration
func (c *Converter) Convert() (*server.MCPServer, error) {
	if c.parser.GetDocument() == nil {
		return nil, errors.New("no OpenAPI document loaded")
	}

	// Create the MCP configuration
	server := server.NewMCPServer(
		c.options.ServerName,
		c.options.Version,
	)

	// Process each path and operation
	for path, pathItem := range c.parser.GetPaths().Map() {
		operations := getOperations(pathItem)
		for method, operation := range operations {
			tool, err := c.convertOperation(path, method, operation)
			if err != nil {
				return nil, fmt.Errorf("failed to convert operation %s %s: %w", method, path, err)
			}
			server.AddTool(*tool, nil)
		}
	}

	return server, nil
}

// getOperations returns a map of HTTP method to operation
func getOperations(pathItem *openapi3.PathItem) map[string]*openapi3.Operation {
	operations := make(map[string]*openapi3.Operation)

	if pathItem.Get != nil {
		operations["get"] = pathItem.Get
	}
	if pathItem.Post != nil {
		operations["post"] = pathItem.Post
	}
	if pathItem.Put != nil {
		operations["put"] = pathItem.Put
	}
	if pathItem.Delete != nil {
		operations["delete"] = pathItem.Delete
	}
	if pathItem.Options != nil {
		operations["options"] = pathItem.Options
	}
	if pathItem.Head != nil {
		operations["head"] = pathItem.Head
	}
	if pathItem.Patch != nil {
		operations["patch"] = pathItem.Patch
	}
	if pathItem.Trace != nil {
		operations["trace"] = pathItem.Trace
	}

	return operations
}

// convertOperation converts an OpenAPI operation to an MCP tool
func (c *Converter) convertOperation(path, method string, operation *openapi3.Operation) (*mcp.Tool, error) {
	// Generate a tool name
	toolName := c.parser.GetOperationID(path, method, operation)
	if c.options.ToolNamePrefix != "" {
		toolName = c.options.ToolNamePrefix + toolName
	}

	args, err := c.convertParameters(operation.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to convert parameters: %w", err)
	}

	args = append(args, mcp.WithDescription(getDescription(operation)))

	tool := mcp.NewTool(toolName,
		args...,
	)

	return &tool, nil
}

type propertyType string

const (
	PropertyTypeString  propertyType = "string"
	PropertyTypeInteger propertyType = "integer"
	PropertyTypeNumber  propertyType = "number"
	PropertyTypeBoolean propertyType = "boolean"
	PropertyTypeObject  propertyType = "object"
	PropertyTypeArray   propertyType = "array"
)

// convertParameters converts OpenAPI parameters to MCP arguments
func (c *Converter) convertParameters(parameters openapi3.Parameters) ([]mcp.ToolOption, error) {
	args := []mcp.ToolOption{}

	for _, paramRef := range parameters {
		param := paramRef.Value
		if param == nil {
			continue
		}

		propertyOptions := []mcp.PropertyOption{
			mcp.Description(param.Description),
		}

		if param.Required {
			propertyOptions = append(propertyOptions, mcp.Required())
		}

		t := PropertyTypeString
		if param.Schema != nil && param.Schema.Value != nil {
			schema := param.Schema.Value

			// Determine property type and add specific options
			if schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
				t = PropertyTypeArray
				item := c.processSchemaItems(schema.Items.Value)
				propertyOptions = append(propertyOptions, mcp.Items(item))
			} else if schema.Type.Is("object") && len(schema.Properties) > 0 {
				t = PropertyTypeObject
				obj := c.processSchemaProperties(schema)
				propertyOptions = append(propertyOptions, mcp.Properties(obj))
			} else if schema.Type.Is("integer") {
				t = PropertyTypeInteger
			} else if schema.Type.Is("number") {
				t = PropertyTypeNumber
			} else if schema.Type.Is("boolean") {
				t = PropertyTypeBoolean
			}
		}

		// Add the parameter based on its type
		args = append(args, c.createToolOption(t, param.In+"|"+param.Name, propertyOptions...))
	}

	return args, nil
}

// processSchemaItems processes schema items for array types
func (c *Converter) processSchemaItems(schema *openapi3.Schema) map[string]interface{} {
	item := make(map[string]interface{})
	item["type"] = schema.Type

	if schema.Description != "" {
		item["description"] = schema.Description
	}

	// Process nested properties if this is an object
	if len(schema.Properties) > 0 {
		properties := make(map[string]interface{})
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				properties[propName] = c.processSchemaProperty(propRef.Value)
			}
		}
		item["properties"] = properties
	}

	return item
}

// processSchemaProperties processes schema properties for object types
func (c *Converter) processSchemaProperties(schema *openapi3.Schema) map[string]interface{} {
	obj := make(map[string]interface{})

	for propName, propRef := range schema.Properties {
		if propRef.Value != nil {
			obj[propName] = c.processSchemaProperty(propRef.Value)
		}
	}

	return obj
}

// processSchemaProperty processes a single schema property
func (c *Converter) processSchemaProperty(schema *openapi3.Schema) map[string]interface{} {
	property := make(map[string]interface{})
	property["type"] = schema.Type

	if schema.Description != "" {
		property["description"] = schema.Description
	}
	if schema.Default != nil {
		property["default"] = schema.Default
	}
	if len(schema.Enum) > 0 {
		property["enum"] = schema.Enum
	}
	if schema.Format != "" {
		property["format"] = schema.Format
	}
	if schema.Min != nil {
		property["min"] = schema.Min
	}
	if schema.Max != nil {
		property["max"] = schema.Max
	}
	if schema.MinLength != 0 {
		property["minLength"] = schema.MinLength
	}
	if schema.MaxLength != nil {
		property["maxLength"] = *schema.MaxLength
	}
	if schema.MinItems != 0 {
		property["minItems"] = schema.MinItems
	}
	if schema.MaxItems != nil {
		property["maxItems"] = *schema.MaxItems
	}
	if schema.MinProps != 0 {
		property["minProperties"] = schema.MinProps
	}
	if schema.MaxProps != nil {
		property["maxProperties"] = *schema.MaxProps
	}

	// Recursively process nested objects
	if schema.Type.Is("object") && len(schema.Properties) > 0 {
		nestedProps := make(map[string]interface{})
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				nestedProps[propName] = c.processSchemaProperty(propRef.Value)
			}
		}
		property["properties"] = nestedProps
	}

	// Recursively process array items
	if schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
		property["items"] = c.processSchemaItems(schema.Items.Value)
	}

	return property
}

// createToolOption creates the appropriate tool option based on property type
func (c *Converter) createToolOption(t propertyType, name string, options ...mcp.PropertyOption) mcp.ToolOption {
	switch t {
	case PropertyTypeString:
		return mcp.WithString(name, options...)
	case PropertyTypeInteger, PropertyTypeNumber:
		return mcp.WithNumber(name, options...)
	case PropertyTypeBoolean:
		return mcp.WithBoolean(name, options...)
	case PropertyTypeObject:
		return mcp.WithObject(name, options...)
	case PropertyTypeArray:
		return mcp.WithArray(name, options...)
	default:
		return mcp.WithString(name, options...)
	}
}

// getDescription returns a description for an operation
func getDescription(operation *openapi3.Operation) string {
	if operation.Summary != "" {
		if operation.Description != "" {
			return fmt.Sprintf("%s - %s", operation.Summary, operation.Description)
		}
		return operation.Summary
	}
	return operation.Description
}
