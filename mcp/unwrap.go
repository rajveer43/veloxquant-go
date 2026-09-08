package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// unwrapMcpToolResult unwraps an MCP CallToolResult into the plain Go
// value agent.Agent.Run expects to json.Marshal back to the model,
// matching the TS SDK's mcp.ts unwrapMcpToolResult content-handling rules
// exactly:
//
//  1. StructuredContent is preferred when the server provides it (already
//     a plain value, no parsing needed).
//  2. Otherwise, a single text content block is tried as JSON first,
//     falling back to the raw string if it doesn't parse.
//  3. Multiple text content blocks are returned as a []string of their
//     text, in order.
//  4. Any other content type (image, audio, resource, resource_link)
//     returns an explicit, actionable error rather than silently dropping
//     it or guessing how to represent it to a text-only chat model.
//  5. No content at all returns nil.
func unwrapMcpToolResult(result *mcp.CallToolResult) (any, error) {
	if result.StructuredContent != nil {
		return result.StructuredContent, nil
	}

	content := result.Content
	if len(content) == 0 {
		return nil, nil
	}

	for _, c := range content {
		if _, ok := c.(*mcp.TextContent); !ok {
			return nil, fmt.Errorf(
				"mcp tool result contained unsupported content type %s — only \"text\" content "+
					"(or structuredContent) is currently unwrapped into a tool result; image/audio/"+
					"resource content needs a deliberate design decision about how to surface it to "+
					"a text-only chat model",
				contentTypeName(c),
			)
		}
	}

	if len(content) == 1 {
		text := content[0].(*mcp.TextContent).Text
		var parsed any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil {
			return parsed, nil
		}
		return text, nil
	}

	texts := make([]string, len(content))
	for i, c := range content {
		texts[i] = c.(*mcp.TextContent).Text
	}
	return texts, nil
}

// contentTypeName reports the MCP wire "type" discriminator for c, for use
// in the unsupported-content-type error message.
func contentTypeName(c mcp.Content) string {
	switch c.(type) {
	case *mcp.ImageContent:
		return `"image"`
	case *mcp.AudioContent:
		return `"audio"`
	case *mcp.ResourceLink:
		return `"resource_link"`
	case *mcp.EmbeddedResource:
		return `"resource"`
	default:
		return fmt.Sprintf("%T", c)
	}
}

// contentSummary renders a short human-readable summary of content, used
// when an MCP tool call reports IsError: true (a normal, non-exceptional
// tool-level failure, not a transport error) to include in the wrapped Go
// error's message.
func contentSummary(content []mcp.Content) string {
	if len(content) == 0 {
		return "(no content)"
	}
	if text, ok := content[0].(*mcp.TextContent); ok {
		return text.Text
	}
	return contentTypeName(content[0])
}
