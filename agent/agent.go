// Package agent implements a single-turn tool-calling loop over a
// veloxquant.Client: send a prompt, execute any tools the model calls,
// feed the results back, and repeat until the model stops calling tools
// or a maximum number of round trips is used up.
//
// It reuses the OpenAI tools/tool_calls wire shape end to end (see
// openai.ToolDefinition/ToolCall, re-exported at the veloxquant package's
// top level) rather than inventing a bespoke schema, since the underlying
// mlx_lm server (wrapped by `vq serve`) parses tool calls natively against
// the model's own tokenizer/chat template using exactly this shape —
// mirroring the TS SDK's agent.ts, which documents the same design and
// confirms it working end to end against mlx-community/Qwen3-4B-4bit.
//
// This package has no third-party dependency and lives in the root Go
// module. Tool sources: manually registered via RegisterTool, or pulled
// from an MCP server via UseMcpServer (see the separate mcp module) — both
// share one name/dispatch namespace inside Run.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

// defaultMaxSteps is the number of tool-call round trips Run allows before
// giving up, when RunOptions.MaxSteps is left at its zero value.
const defaultMaxSteps = 8

// ErrAgentMaxStepsExceeded is returned (wrapped) by Run when the model
// keeps calling tools past RunOptions.MaxSteps without producing a final,
// tool-call-free response.
var ErrAgentMaxStepsExceeded = errors.New("agent: exceeded max steps")

// Tool is a single callable tool an Agent can dispatch to. It is a plain
// interface (rather than a struct with a function field) so packages
// outside this one — such as the separate mcp module — can implement it
// without agent needing to import them; this keeps the dependency
// direction one-way (mcp imports agent, never the reverse).
type Tool interface {
	// Name is the tool's unique identifier, matching the "name" field of
	// an OpenAI tool-call request/response.
	Name() string
	// Description is a human/model-readable summary of what the tool
	// does, used as the tool definition's "description" field.
	Description() string
	// Parameters returns a JSON Schema object (typically map[string]any)
	// describing the tool's arguments, used as the tool definition's
	// "parameters" field.
	Parameters() any
	// Execute runs the tool against args (the model-supplied arguments,
	// still-encoded JSON) and returns a result that will be
	// JSON-marshaled back to the model as the tool-result message
	// content. Returning an error does not abort Run: the error's
	// message is fed back to the model as a structured error result, and
	// the loop continues (matching agent.ts's tool-error handling).
	Execute(ctx context.Context, args json.RawMessage) (any, error)
}

// RunOptions configures a single Agent.Run call.
type RunOptions struct {
	// MaxSteps caps the number of tool-call round trips before Run gives
	// up and returns an error wrapping ErrAgentMaxStepsExceeded. The zero
	// value resolves to 8, not literally zero — a runaway tool loop stops
	// instead of looping forever, matching agent.ts's default.
	MaxSteps int
	// MaxTokens limits the length of each chat completion. Zero means the
	// runtime's own default.
	MaxTokens int
	// Temperature controls sampling randomness for each chat completion.
	Temperature float64
}

func (o RunOptions) maxSteps() int {
	if o.MaxSteps <= 0 {
		return defaultMaxSteps
	}
	return o.MaxSteps
}

// Step records one executed tool call within a Run.
type Step struct {
	ToolName string
	Args     any
	Result   any
}

// RunResult is the outcome of a completed Agent.Run call.
type RunResult struct {
	// Text is the model's final, tool-call-free response.
	Text string
	// Steps records every tool call executed during the run, in order.
	Steps []Step
}

// Agent is a single-turn tool-calling agent over a veloxquant.Client and a
// specific model. Construct one with New, register tools with
// RegisterTool (and/or UseMcpServer), then call Run. The zero value is not
// usable.
type Agent struct {
	client *veloxquant.Client
	model  string

	tools     map[string]Tool
	toolOrder []string
	sources   []ToolSource
}

// New returns an Agent that sends chat completions to model via client.
func New(client *veloxquant.Client, model string) *Agent {
	return &Agent{
		client: client,
		model:  model,
		tools:  make(map[string]Tool),
	}
}

// Close closes every ToolSource this agent accumulated via UseMcpServer
// (an MCP source built from a caller-supplied, already-connected client is
// expected to make its own Close a no-op, since that connection's
// lifecycle belongs to whoever created it — the same ownership rule
// mcp.ToolSource documents). It does not touch the underlying
// veloxquant.Client, whose lifecycle this package doesn't own.
func (a *Agent) Close(ctx context.Context) error {
	var firstErr error
	for _, s := range a.sources {
		if err := s.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RegisterTool adds t to this agent's tool set. It returns an error,
// rather than panicking, if a tool with the same Name() is already
// registered — whether that tool was registered manually or came from an
// MCP server via UseMcpServer, since MCP tools aren't treated as
// second-class here.
func (a *Agent) RegisterTool(t Tool) error {
	name := t.Name()
	if _, exists := a.tools[name]; exists {
		return fmt.Errorf("agent: a tool named %q is already registered", name)
	}
	a.tools[name] = t
	a.toolOrder = append(a.toolOrder, name)
	return nil
}

// toolDefinitions builds the OpenAI tool-definition list for every
// currently registered tool, in registration order (for deterministic
// wire output).
func (a *Agent) toolDefinitions() []veloxquant.ToolDefinition {
	defs := make([]veloxquant.ToolDefinition, 0, len(a.toolOrder))
	for _, name := range a.toolOrder {
		t := a.tools[name]
		defs = append(defs, veloxquant.ToolDefinition{
			Type: "function",
			Function: veloxquant.ToolFunction{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// toolErrorJSON marshals a structured {"error": msg} result, used both for
// dispatch failures (unknown tool, malformed arguments) and tool execution
// errors. Marshaling a plain string can't fail, so the error return here
// is only for defensive completeness.
func toolErrorJSON(msg string) string {
	data, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		// Unreachable in practice: map[string]string always marshals.
		return `{"error":"internal error building error response"}`
	}
	return string(data)
}

// Run sends prompt as a user message, executing any tools the model calls
// and feeding their results back, until the model responds without
// calling a tool or opts.MaxSteps round trips are used up (whichever
// comes first).
//
// A tool call naming a tool that isn't registered, or whose arguments
// aren't valid JSON, does not abort the run: a structured {"error": "..."}
// result is fed back to the model as that call's result and the loop
// continues, matching agent.ts's malformed-input resilience. A tool's
// Execute returning an error is handled the same way, using the error's
// message.
func (a *Agent) Run(ctx context.Context, prompt string, opts RunOptions) (RunResult, error) {
	maxSteps := opts.maxSteps()
	tools := a.toolDefinitions()

	messages := []veloxquant.Message{{Role: "user", Content: prompt}}
	var steps []Step

	for step := 0; step < maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return RunResult{}, err
		}

		req := veloxquant.ChatRequest{
			Model:       a.model,
			Messages:    messages,
			MaxTokens:   opts.MaxTokens,
			Temperature: opts.Temperature,
		}
		if len(tools) > 0 {
			req.Tools = tools
		}

		resp, err := a.client.Chat(ctx, req)
		if err != nil {
			return RunResult{}, fmt.Errorf("agent: chat request failed: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			return RunResult{Text: resp.Text, Steps: steps}, nil
		}

		messages = append(messages, veloxquant.Message{
			Role:      "assistant",
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			tool, ok := a.tools[call.Function.Name]
			if !ok {
				messages = append(messages, veloxquant.Message{
					Role:       "tool",
					ToolCallID: call.ID,
					Content:    toolErrorJSON(fmt.Sprintf("No tool named %q is registered.", call.Function.Name)),
				})
				continue
			}

			var args json.RawMessage
			if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				messages = append(messages, veloxquant.Message{
					Role:       "tool",
					ToolCallID: call.ID,
					Content:    toolErrorJSON(fmt.Sprintf("Could not parse arguments as JSON: %s", call.Function.Arguments)),
				})
				continue
			}

			result, err := tool.Execute(ctx, args)
			if err != nil {
				result = map[string]string{"error": err.Error()}
			}

			resultJSON, err := json.Marshal(result)
			if err != nil {
				resultJSON = []byte(toolErrorJSON(fmt.Sprintf("failed to encode tool result: %s", err.Error())))
			}

			var decodedArgs any
			_ = json.Unmarshal(args, &decodedArgs)

			steps = append(steps, Step{ToolName: call.Function.Name, Args: decodedArgs, Result: result})
			messages = append(messages, veloxquant.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    string(resultJSON),
			})
		}
	}

	return RunResult{}, fmt.Errorf("agent: exceeded max steps (%d): %w", maxSteps, ErrAgentMaxStepsExceeded)
}
