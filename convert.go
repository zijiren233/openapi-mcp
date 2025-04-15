package main

import (
	"errors"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ConvertOptions struct {
	ToolNamePrefix string
}

// Converter represents an OpenAPI to MCP converter
type Converter struct {
	parser  *Parser
	options ConvertOptions
}

// NewConverter creates a new OpenAPI to MCP converter
func NewConverter(parser *Parser, options ConvertOptions) *Converter {
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

	info := c.parser.GetInfo()
	if info == nil {
		return nil, errors.New("no info found in OpenAPI document")
	}

	// Create the MCP configuration
	server := server.NewMCPServer(
		info.Title,
		info.Version,
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

	// Handle request body if present
	// if operation.RequestBody != nil && operation.RequestBody.Value != nil {
	// 	bodyArgs, err := c.convertRequestBody(operation.RequestBody.Value)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("failed to convert request body: %w", err)
	// 	}
	// 	args = append(args, bodyArgs...)
	// }

	// Add server address parameter
	servers := c.parser.GetServers()
	if len(servers) == 0 {
		args = append(args, mcp.WithString("openapi_server_addr",
			mcp.Description("Server address to connect to"),
			mcp.Required()))
	} else if len(servers) != 1 {
		serverUrls := make([]string, 0, len(servers))
		for _, server := range servers {
			serverUrls = append(serverUrls, server.URL)
		}
		args = append(args, mcp.WithString("openapi_server_addr",
			mcp.Description("Server address to connect to"),
			mcp.Required(),
			mcp.Enum(serverUrls...)))
	}

	args = append(args, mcp.WithDescription(getDescription(operation)))

	tool := mcp.NewTool(toolName,
		args...,
	)

	return &tool, nil
}

// convertRequestBody converts an OpenAPI request body to MCP arguments
func (c *Converter) convertRequestBody(requestBody *openapi3.RequestBody) ([]mcp.ToolOption, error) {
	args := []mcp.ToolOption{}

	for contentType, mediaType := range requestBody.Content {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		schema := mediaType.Schema.Value
		propertyOptions := []mcp.PropertyOption{}

		if requestBody.Description != "" {
			propertyOptions = append(propertyOptions, mcp.Description(requestBody.Description))
		}

		if requestBody.Required {
			propertyOptions = append(propertyOptions, mcp.Required())
		}

		t := PropertyTypeObject
		if schema.Type != nil {
			if schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
				t = PropertyTypeArray
				item := c.processSchemaItems(schema.Items.Value)
				propertyOptions = append(propertyOptions, mcp.Items(item))
			} else if schema.Type.Is("object") || len(schema.Properties) > 0 {
				obj := c.processSchemaProperties(schema)
				propertyOptions = append(propertyOptions, mcp.Properties(obj))
			} else if schema.Type.Is("string") {
				t = PropertyTypeString
			} else if schema.Type.Is("integer") {
				t = PropertyTypeInteger
			} else if schema.Type.Is("number") {
				t = PropertyTypeNumber
			} else if schema.Type.Is("boolean") {
				t = PropertyTypeBoolean
			}
		}

		// Add content type as part of the parameter name
		paramName := "body|" + contentType
		args = append(args, c.createToolOption(t, paramName, propertyOptions...))
	}

	return args, nil
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

	if schema.Type != nil {
		item["type"] = schema.Type
	}

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

	// Handle reference if this is a reference to another schema
	if schema.Items != nil && schema.Items.Value != nil {
		item["items"] = c.processSchemaItems(schema.Items.Value)
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

	if schema.Type != nil {
		property["type"] = schema.Type
	}

	// Basic metadata
	if schema.Title != "" {
		property["title"] = schema.Title
	}
	if schema.Description != "" {
		property["description"] = schema.Description
	}
	if schema.Default != nil {
		property["default"] = schema.Default
	}
	if schema.Example != nil {
		property["example"] = schema.Example
	}
	if len(schema.Enum) > 0 {
		property["enum"] = schema.Enum
	}
	if schema.Format != "" {
		property["format"] = schema.Format
	}

	// Schema composition
	if len(schema.OneOf) > 0 {
		oneOf := make([]interface{}, 0, len(schema.OneOf))
		for _, schemaRef := range schema.OneOf {
			if schemaRef.Value != nil {
				oneOf = append(oneOf, c.processSchemaProperty(schemaRef.Value))
			}
		}
		if len(oneOf) > 0 {
			property["oneOf"] = oneOf
		}
	}

	if len(schema.AnyOf) > 0 {
		anyOf := make([]interface{}, 0, len(schema.AnyOf))
		for _, schemaRef := range schema.AnyOf {
			if schemaRef.Value != nil {
				anyOf = append(anyOf, c.processSchemaProperty(schemaRef.Value))
			}
		}
		if len(anyOf) > 0 {
			property["anyOf"] = anyOf
		}
	}

	if len(schema.AllOf) > 0 {
		allOf := make([]interface{}, 0, len(schema.AllOf))
		for _, schemaRef := range schema.AllOf {
			if schemaRef.Value != nil {
				allOf = append(allOf, c.processSchemaProperty(schemaRef.Value))
			}
		}
		if len(allOf) > 0 {
			property["allOf"] = allOf
		}
	}

	if schema.Not != nil && schema.Not.Value != nil {
		property["not"] = c.processSchemaProperty(schema.Not.Value)
	}

	// Boolean flags
	if schema.Nullable {
		property["nullable"] = schema.Nullable
	}
	if schema.ReadOnly {
		property["readOnly"] = schema.ReadOnly
	}
	if schema.WriteOnly {
		property["writeOnly"] = schema.WriteOnly
	}
	if schema.Deprecated {
		property["deprecated"] = schema.Deprecated
	}
	if schema.AllowEmptyValue {
		property["allowEmptyValue"] = schema.AllowEmptyValue
	}
	if schema.UniqueItems {
		property["uniqueItems"] = schema.UniqueItems
	}
	if schema.ExclusiveMin {
		property["exclusiveMinimum"] = schema.ExclusiveMin
	}
	if schema.ExclusiveMax {
		property["exclusiveMaximum"] = schema.ExclusiveMax
	}

	// Number validations
	if schema.Min != nil {
		property["minimum"] = *schema.Min
	}
	if schema.Max != nil {
		property["maximum"] = *schema.Max
	}
	if schema.MultipleOf != nil {
		property["multipleOf"] = *schema.MultipleOf
	}

	// String validations
	if schema.MinLength != 0 {
		property["minLength"] = schema.MinLength
	}
	if schema.MaxLength != nil {
		property["maxLength"] = *schema.MaxLength
	}
	if schema.Pattern != "" {
		property["pattern"] = schema.Pattern
	}

	// Array validations
	if schema.MinItems != 0 {
		property["minItems"] = schema.MinItems
	}
	if schema.MaxItems != nil {
		property["maxItems"] = *schema.MaxItems
	}

	// Object validations
	if schema.MinProps != 0 {
		property["minProperties"] = schema.MinProps
	}
	if schema.MaxProps != nil {
		property["maxProperties"] = *schema.MaxProps
	}
	if len(schema.Required) > 0 {
		property["required"] = schema.Required
	}

	// Handle AdditionalProperties
	if schema.AdditionalProperties.Has != nil {
		property["additionalProperties"] = *schema.AdditionalProperties.Has
	} else if schema.AdditionalProperties.Schema != nil && schema.AdditionalProperties.Schema.Value != nil {
		property["additionalProperties"] = c.processSchemaProperty(schema.AdditionalProperties.Schema.Value)
	}

	// Handle discriminator
	if schema.Discriminator != nil {
		discriminator := make(map[string]interface{})
		discriminator["propertyName"] = schema.Discriminator.PropertyName
		if len(schema.Discriminator.Mapping) > 0 {
			discriminator["mapping"] = schema.Discriminator.Mapping
		}
		property["discriminator"] = discriminator
	}

	// Recursively process nested objects
	if schema.Type != nil && schema.Type.Is("object") && len(schema.Properties) > 0 {
		nestedProps := make(map[string]interface{})
		for propName, propRef := range schema.Properties {
			if propRef.Value != nil {
				nestedProps[propName] = c.processSchemaProperty(propRef.Value)
			}
		}
		property["properties"] = nestedProps
	}

	// Recursively process array items
	if schema.Type != nil && schema.Type.Is("array") && schema.Items != nil && schema.Items.Value != nil {
		property["items"] = c.processSchemaItems(schema.Items.Value)
	}

	// Handle external docs if present
	if schema.ExternalDocs != nil {
		externalDocs := make(map[string]interface{})
		if schema.ExternalDocs.Description != "" {
			externalDocs["description"] = schema.ExternalDocs.Description
		}
		if schema.ExternalDocs.URL != "" {
			externalDocs["url"] = schema.ExternalDocs.URL
		}
		property["externalDocs"] = externalDocs
	}

	// Handle XML object if present
	if schema.XML != nil {
		xml := make(map[string]interface{})
		if schema.XML.Name != "" {
			xml["name"] = schema.XML.Name
		}
		if schema.XML.Namespace != "" {
			xml["namespace"] = schema.XML.Namespace
		}
		if schema.XML.Prefix != "" {
			xml["prefix"] = schema.XML.Prefix
		}
		xml["attribute"] = schema.XML.Attribute
		xml["wrapped"] = schema.XML.Wrapped
		property["xml"] = xml
	}

	return property
}

// createToolOption creates the appropriate tool option based on property type
func (c *Converter) createToolOption(t propertyType, name string, options ...mcp.PropertyOption) mcp.ToolOption {
	switch t {
	case PropertyTypeString:
		return mcp.WithString(name, options...)
	case PropertyTypeInteger:
		return mcp.WithNumber(name, options...)
	case PropertyTypeNumber:
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
