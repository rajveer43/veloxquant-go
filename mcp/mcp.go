// Package mcp lets a github.com/rajveer43/veloxquant-go/agent.Agent pull
// tools from a Model Context Protocol (MCP) server, backed by the official
// github.com/modelcontextprotocol/go-sdk.
//
// This package is a separate Go module from both the root veloxquant-go
// module and the dependency-free agent package, specifically so that
// importing it (and its MCP SDK dependency) is opt-in — exactly like
// langchain/ isolates langchaingo. Depends on agent (mcp imports agent,
// never the reverse) only for the agent.Tool interface, which ToolSource's
// tools satisfy.
//
// API-shape divergence from the TS SDK: TS's Agent.useMcpServer(config)
// connects to the server itself, via a dynamic import('./mcp.js') so that
// importing agent.ts doesn't force every caller to depend on the MCP SDK.
// Go has no equivalent runtime-lazy import, so the same dependency
// isolation is instead achieved at the module boundary: this package does
// the connecting (Connect), and agent.Agent.UseMcpServer takes an
// already-built agent.ToolSource rather than a config struct the agent
// package would need to know how to connect itself. This is a deliberate,
// considered difference from TS's API shape — not an incomplete port —
// and is the only way to keep the agent package (and the root module) free
// of any MCP SDK dependency while still letting an Agent accept
// MCP-sourced tools.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rajveer43/veloxquant-go/agent"
)

// Transport selects which MCP transport ServerConfig connects with.
type Transport int

const (
	// TransportStdio launches Command as a subprocess and speaks MCP over
	// its stdin/stdout.
	TransportStdio Transport = iota
	// TransportSSE connects to an MCP server over Server-Sent Events at
	// URL.
	TransportSSE
	// TransportHTTP connects to an MCP server over the streamable HTTP
	// transport at URL.
	TransportHTTP
)

// ServerConfig describes how to connect to an MCP server. Exactly one of
// the transport-specific field groups applies, selected by Transport:
// TransportStdio uses Command/Args/Env; TransportSSE and TransportHTTP use
// URL. A Go-idiomatic tagged-variant shape (struct + enum field) was
// chosen over an interface-per-transport, matching the flatter style
// RunOptions and similar option structs already use elsewhere in this
// SDK family, rather than introducing a new interface-per-variant pattern
// for a single use case.
type ServerConfig struct {
	// Name identifies this server in error messages (e.g. a tool-name
	// collision report).
	Name string
	// Transport selects which of the fields below apply.
	Transport Transport

	// Command, Args, and Env configure TransportStdio: the subprocess to
	// launch and connect to over stdin/stdout.
	Command string
	Args    []string
	Env     []string

	// URL configures TransportSSE and TransportHTTP: the endpoint to
	// connect to.
	URL string
}

// ToolSource connects to an MCP server (or wraps an already-connected
// client) and exposes its tools as agent.Tool values, satisfying
// agent.ToolSource so it can be passed directly to
// (*agent.Agent).UseMcpServer.
type ToolSource struct {
	name           string
	session        *mcp.ClientSession
	ownsConnection bool
}

var _ agent.ToolSource = (*ToolSource)(nil)

// Connect connects to the MCP server described by cfg and returns a
// ToolSource that owns the resulting connection: Close will close it.
func Connect(ctx context.Context, cfg ServerConfig) (*ToolSource, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "veloxquant-go-agent-" + cfg.Name, Version: "1"}, nil)

	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, fmt.Errorf("mcp: %w", err)
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: connect to server %q: %w", cfg.Name, err)
	}

	return &ToolSource{name: cfg.Name, session: session, ownsConnection: true}, nil
}

// FromSession wraps an already-connected *mcp.ClientSession as a
// ToolSource. Unlike Connect, the returned ToolSource does not own the
// connection: Close is a no-op, since the session's lifecycle belongs to
// whoever created it — the same caller-owns-the-connection rule the TS SDK
// documents for its equivalent case (mcp.ts's { client: Client } config
// variant).
func FromSession(name string, session *mcp.ClientSession) *ToolSource {
	return &ToolSource{name: name, session: session, ownsConnection: false}
}

func buildTransport(cfg ServerConfig) (mcp.Transport, error) {
	switch cfg.Transport {
	case TransportStdio:
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires Command")
		}
		cmd := exec.Command(cfg.Command, cfg.Args...)
		if len(cfg.Env) > 0 {
			cmd.Env = cfg.Env
		}
		return &mcp.CommandTransport{Command: cmd}, nil
	case TransportSSE:
		if cfg.URL == "" {
			return nil, fmt.Errorf("sse transport requires URL")
		}
		return &mcp.SSEClientTransport{Endpoint: cfg.URL}, nil
	case TransportHTTP:
		if cfg.URL == "" {
			return nil, fmt.Errorf("http transport requires URL")
		}
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL}, nil
	default:
		return nil, fmt.Errorf("unknown transport %d", cfg.Transport)
	}
}

// ListTools returns this source's tools as agent.Tool values, each
// dispatching Execute through this source's MCP session.
func (s *ToolSource) ListTools(ctx context.Context) ([]agent.Tool, error) {
	result, err := s.session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcp: list tools from server %q: %w", s.name, err)
	}

	tools := make([]agent.Tool, 0, len(result.Tools))
	for _, t := range result.Tools {
		tools = append(tools, &mcpTool{session: s.session, tool: t})
	}
	return tools, nil
}

// Close closes the underlying MCP connection if this ToolSource opened it
// itself (via Connect). If it was built from an already-connected session
// via FromSession, Close is a no-op: that connection's lifecycle belongs
// to whoever created it.
func (s *ToolSource) Close(ctx context.Context) error {
	if !s.ownsConnection {
		return nil
	}
	return s.session.Close()
}

// mcpTool adapts a single MCP tool to agent.Tool, dispatching Execute as
// an MCP CallTool request over session.
type mcpTool struct {
	session *mcp.ClientSession
	tool    *mcp.Tool
}

var _ agent.Tool = (*mcpTool)(nil)

func (t *mcpTool) Name() string        { return t.tool.Name }
func (t *mcpTool) Description() string { return t.tool.Description }
func (t *mcpTool) Parameters() any     { return t.tool.InputSchema }

func (t *mcpTool) Execute(ctx context.Context, args json.RawMessage) (any, error) {
	var arguments any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("mcp: decode arguments for tool %q: %w", t.tool.Name, err)
		}
	}

	result, err := t.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      t.tool.Name,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("mcp: call tool %q: %w", t.tool.Name, err)
	}
	if result.IsError {
		return nil, fmt.Errorf("mcp: tool %q reported an error: %s", t.tool.Name, contentSummary(result.Content))
	}

	return unwrapMcpToolResult(result)
}
