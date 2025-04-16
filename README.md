# OpenAPI MCP

`openapi-mcp` is a tool that converts OpenAPI specifications into MCP (Machine Chat Protocol) servers, enabling seamless integration of API definitions with AI assistants.

## Features

- Supports both OpenAPI v2 (Swagger) and OpenAPI v3 specifications
- Converts API endpoints into MCP tools
- Provides both StdIO and SSE server modes

## How to use

### StdIO

```bash
go run . --file doc.json
```

### SSE

```bash
# serve on http://localhost:3000/sse
go run . --file doc.json --sse 0.0.0.0:3000
```
