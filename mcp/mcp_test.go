package mcp

import (
	"context"
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rajveer43/veloxquant-go/agent"
)

// newTestSession starts an in-memory MCP server with the given tools
// registered and returns a connected client session. Cleanup closes it.
func newTestSession(t *testing.T, register func(s *sdkmcp.Server)) *sdkmcp.ClientSession {
	t.Helper()

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test-server", Version: "1"}, nil)
	register(server)

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()

	ctx := context.Background()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// newTestServer returns a ToolSource wrapping a fresh in-memory session via
// FromSession — i.e. one that does NOT own its connection, matching the
// caller-owns-the-connection case. Used by tests that only care about tool
// listing/execution/unwrapping, not connection-ownership semantics.
func newTestServer(t *testing.T, register func(s *sdkmcp.Server)) *ToolSource {
	t.Helper()
	session := newTestSession(t, register)
	return FromSession("test-server", session)
}

// newOwningTestServer returns a ToolSource that owns its connection (as if
// built via Connect), so Close actually closes the session — needed to
// test connection-ownership and collision-rollback behavior.
func newOwningTestServer(t *testing.T, register func(s *sdkmcp.Server)) *ToolSource {
	t.Helper()
	session := newTestSession(t, register)
	return &ToolSource{name: "test-server", session: session, ownsConnection: true}
}

func textResult(text string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: text}}}
}

func TestListToolsReturnsAgentTools(t *testing.T) {
	source := newTestServer(t, func(s *sdkmcp.Server) {
		s.AddTool(&sdkmcp.Tool{
			Name:        "echo",
			Description: "Echoes input",
			InputSchema: map[string]any{"type": "object"},
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return textResult("echoed"), nil
		})
	})

	tools, err := source.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("len(tools) = %d, want 1", len(tools))
	}
	if tools[0].Name() != "echo" {
		t.Errorf("Name() = %q, want %q", tools[0].Name(), "echo")
	}
	if tools[0].Description() != "Echoes input" {
		t.Errorf("Description() = %q", tools[0].Description())
	}
}

func TestExecuteRoundTrip(t *testing.T) {
	source := newTestServer(t, func(s *sdkmcp.Server) {
		s.AddTool(&sdkmcp.Tool{
			Name:        "add",
			Description: "Adds two numbers",
			InputSchema: map[string]any{"type": "object"},
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			var args struct {
				A, B float64
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
			return &sdkmcp.CallToolResult{StructuredContent: map[string]any{"sum": args.A + args.B}}, nil
		})
	})

	tools, err := source.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{"a":2,"b":3}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result = %#v, want map[string]any", result)
	}
	if m["sum"] != float64(5) {
		t.Errorf("sum = %v, want 5", m["sum"])
	}
}

func TestUnwrapPrefersStructuredContent(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: `{"ignored":true}`}},
		StructuredContent: map[string]any{"preferred": true},
	}
	got, err := unwrapMcpToolResult(result)
	if err != nil {
		t.Fatalf("unwrapMcpToolResult() error = %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok || m["preferred"] != true {
		t.Errorf("got = %#v, want structuredContent", got)
	}
}

func TestUnwrapSingleTextBlockParsedAsJSON(t *testing.T) {
	got, err := unwrapMcpToolResult(textResult(`{"a":1}`))
	if err != nil {
		t.Fatalf("unwrapMcpToolResult() error = %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok || m["a"] != float64(1) {
		t.Errorf("got = %#v, want parsed JSON object", got)
	}
}

func TestUnwrapSingleTextBlockFallsBackToRawString(t *testing.T) {
	got, err := unwrapMcpToolResult(textResult("just plain text, not json"))
	if err != nil {
		t.Fatalf("unwrapMcpToolResult() error = %v", err)
	}
	if got != "just plain text, not json" {
		t.Errorf("got = %#v", got)
	}
}

func TestUnwrapMultipleTextBlocks(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: "first"},
			&sdkmcp.TextContent{Text: "second"},
		},
	}
	got, err := unwrapMcpToolResult(result)
	if err != nil {
		t.Fatalf("unwrapMcpToolResult() error = %v", err)
	}
	texts, ok := got.([]string)
	if !ok || len(texts) != 2 || texts[0] != "first" || texts[1] != "second" {
		t.Errorf("got = %#v", got)
	}
}

func TestUnwrapNoContent(t *testing.T) {
	got, err := unwrapMcpToolResult(&sdkmcp.CallToolResult{})
	if err != nil {
		t.Fatalf("unwrapMcpToolResult() error = %v", err)
	}
	if got != nil {
		t.Errorf("got = %#v, want nil", got)
	}
}

func TestUnwrapUnsupportedContentTypeReturnsActionableError(t *testing.T) {
	result := &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{&sdkmcp.ImageContent{Data: []byte{1, 2, 3}, MIMEType: "image/png"}},
	}
	_, err := unwrapMcpToolResult(result)
	if err == nil {
		t.Fatal("expected an error for unsupported content type")
	}
	if got := err.Error(); !contains(got, "image") {
		t.Errorf("error = %q, want it to mention the unsupported type", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

func TestConnectionOwnershipCloseIsNoOpForFromSession(t *testing.T) {
	source := newTestServer(t, func(s *sdkmcp.Server) {})
	// FromSession-built sources don't own the connection: Close must be a
	// no-op, and calling it explicitly here (beyond the t.Cleanup one)
	// must not error.
	if err := source.Close(context.Background()); err != nil {
		t.Errorf("Close() error = %v, want nil (no-op for a session we don't own)", err)
	}
	// The underlying session must still be usable: Close being a no-op
	// means the connection genuinely wasn't touched.
	if _, err := source.session.ListTools(context.Background(), nil); err != nil {
		t.Errorf("ListTools() after no-op Close error = %v, want the session to remain open", err)
	}
}

func TestConnectionOwnershipCloseActuallyClosesForConnect(t *testing.T) {
	source := newOwningTestServer(t, func(s *sdkmcp.Server) {})
	if err := source.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := source.session.ListTools(context.Background(), nil); err == nil {
		t.Error("expected ListTools to fail after Close on an owned connection")
	}
}

func TestUseMcpServerRegistersToolsIndistinguishablyFromManual(t *testing.T) {
	source := newTestServer(t, func(s *sdkmcp.Server) {
		s.AddTool(&sdkmcp.Tool{
			Name:        "mcp_tool",
			Description: "from mcp",
			InputSchema: map[string]any{"type": "object"},
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return textResult("mcp result"), nil
		})
	})

	a := agent.New(nil, "test-model")
	if err := a.UseMcpServer(context.Background(), source); err != nil {
		t.Fatalf("UseMcpServer() error = %v", err)
	}

	// Registering a manual tool with the same name must now collide, just
	// as it would against another manually-registered tool.
	err := a.RegisterTool(&collidingTool{name: "mcp_tool"})
	if err == nil {
		t.Fatal("expected collision error registering a tool with the same name as an MCP tool")
	}
}

func TestUseMcpServerCollisionRollsBackConnection(t *testing.T) {
	source := newOwningTestServer(t, func(s *sdkmcp.Server) {
		s.AddTool(&sdkmcp.Tool{
			Name:        "dup",
			Description: "x",
			InputSchema: map[string]any{"type": "object"},
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return textResult("x"), nil
		})
	})

	a := agent.New(nil, "test-model")
	if err := a.RegisterTool(&collidingTool{name: "dup"}); err != nil {
		t.Fatalf("RegisterTool() error = %v", err)
	}

	err := a.UseMcpServer(context.Background(), source)
	if err == nil {
		t.Fatal("expected a name-collision error from UseMcpServer")
	}

	// The connection must have been closed as part of the rollback: a
	// subsequent ListTools call should fail.
	if _, err := source.session.ListTools(context.Background(), nil); err == nil {
		t.Error("expected ListTools to fail after collision rollback closed the connection")
	}
}

type collidingTool struct{ name string }

func (c *collidingTool) Name() string        { return c.name }
func (c *collidingTool) Description() string { return "collides" }
func (c *collidingTool) Parameters() any     { return map[string]any{"type": "object"} }
func (c *collidingTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	return nil, nil
}
