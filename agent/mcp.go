package agent

import (
	"context"
	"fmt"
)

// ToolSource is anything that can list a set of Tools and later be closed
// — the shape the separate mcp module's mcp.ToolSource satisfies, without
// this package importing mcp. See UseMcpServer's doc comment for why the
// API is shaped this way.
type ToolSource interface {
	// ListTools returns the tools this source currently exposes.
	ListTools(ctx context.Context) ([]Tool, error)
	// Close releases the source's resources (e.g. an MCP connection this
	// source opened itself). Implementations that don't own their
	// underlying connection may make this a no-op.
	Close(ctx context.Context) error
}

// UseMcpServer registers every tool exposed by source alongside any
// manually-registered ones (via RegisterTool) or tools from other sources,
// so a long-lived agent can pick up more tools mid-session.
//
// API-shape divergence from the TS SDK: TS's useMcpServer(config) connects
// to an MCP server itself, using a dynamic import of its mcp.ts module (an
// optional peer dependency) so that importing agent.ts doesn't force every
// caller to also depend on the MCP SDK. Go has no equivalent
// runtime-lazy/dynamic import, so the same dependency-isolation goal can't
// be reached the same way. Instead, UseMcpServer takes an
// already-constructed ToolSource — built by calling into the separate mcp
// module yourself (see that module's package doc) — rather than a config
// struct this package would need to know how to connect. This keeps the
// agent package free of any MCP SDK dependency (and out of the root
// module's dependency graph) while still letting an Agent accept
// MCP-sourced tools; it is a deliberate, considered difference from TS's
// API shape, not an incomplete port.
//
// On a tool-name collision with an already-registered tool (manual or
// from another source), UseMcpServer closes source before returning the
// error, so a rejected registration doesn't leak the connection it just
// opened — matching the TS SDK's collision-with-rollback behavior.
func (a *Agent) UseMcpServer(ctx context.Context, source ToolSource) error {
	newTools, err := source.ListTools(ctx)
	if err != nil {
		_ = source.Close(ctx)
		return fmt.Errorf("agent: list tools from MCP source: %w", err)
	}

	for _, t := range newTools {
		if _, exists := a.tools[t.Name()]; exists {
			_ = source.Close(ctx)
			return fmt.Errorf("agent: a tool named %q is already registered (also declared by this MCP source)", t.Name())
		}
	}

	for _, t := range newTools {
		a.tools[t.Name()] = t
		a.toolOrder = append(a.toolOrder, t.Name())
	}
	a.sources = append(a.sources, source)
	return nil
}
