package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// resultJSON marshals data to a JSON text MCP result.
func resultJSON(data any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal response: %s", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}
