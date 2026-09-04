package veloxquant

import (
	"context"
	"fmt"
)

// Conversation accumulates chat history across turns so callers don't have
// to build []Message by hand on every call. It is not safe for concurrent
// use by multiple goroutines.
type Conversation struct {
	client   *Client
	model    string
	messages []Message
}

// NewConversation returns a Conversation bound to model. Use system to seed
// an initial system prompt, or leave it empty to start with no history.
func (c *Client) NewConversation(model string, system string) *Conversation {
	conv := &Conversation{client: c, model: model}
	if system != "" {
		conv.messages = append(conv.messages, Message{Role: "system", Content: system})
	}
	return conv
}

// Conversation returns a history-tracking Conversation bound to the model
// AutoPilot selected for this Session.
func (s *Session) Conversation(system string) *Conversation {
	return s.client.NewConversation(s.plan.SelectedModel, system)
}

// History returns the accumulated messages so far, oldest first. The
// returned slice is a copy; mutating it does not affect the Conversation.
func (conv *Conversation) History() []Message {
	history := make([]Message, len(conv.messages))
	copy(history, conv.messages)
	return history
}

// Send appends prompt as a user message, sends the full history so far to
// the model, and appends the assistant's reply to the history before
// returning it. If the request fails, the history is left exactly as it
// was before Send was called (the failed turn is not recorded), so a
// retried Send starts from the same state.
func (conv *Conversation) Send(ctx context.Context, prompt string) (ChatResponse, error) {
	pending := append(conv.History(), Message{Role: "user", Content: prompt})

	resp, err := conv.client.Chat(ctx, ChatRequest{
		Model:    conv.model,
		Messages: pending,
	})
	if err != nil {
		return ChatResponse{}, fmt.Errorf("conversation send: %w", err)
	}

	conv.messages = append(pending, Message{Role: "assistant", Content: resp.Text})
	return resp, nil
}

// SendStream appends prompt as a user message and starts a streaming reply.
// Call Drain (or otherwise fully consume the stream and check its Err)
// before the returned history reflects the assistant's reply — the
// assistant message is appended to history only once the stream finishes
// without error. If the stream errors, the history is left as it was
// before SendStream was called.
func (conv *Conversation) SendStream(ctx context.Context, prompt string) (*ConversationStream, error) {
	pending := append(conv.History(), Message{Role: "user", Content: prompt})

	stream, err := conv.client.ChatStream(ctx, ChatRequest{
		Model:    conv.model,
		Messages: pending,
	})
	if err != nil {
		return nil, fmt.Errorf("conversation send stream: %w", err)
	}

	return &ConversationStream{
		inner:   stream,
		conv:    conv,
		pending: pending,
	}, nil
}

// ConversationStream is a streaming reply within a Conversation. It wraps
// ChatStream and, once the stream finishes successfully, commits the
// assistant's full reply to the owning Conversation's history.
type ConversationStream struct {
	inner   *ChatStream
	conv    *Conversation
	pending []Message
	text    []byte
	done    bool
}

// Next advances the stream. It returns false when the stream ends (check
// Err for failures).
func (cs *ConversationStream) Next() bool {
	ok := cs.inner.Next()
	if ok {
		cs.text = append(cs.text, cs.inner.Chunk().Text...)
		return true
	}
	if cs.inner.Err() == nil && !cs.done {
		cs.done = true
		cs.conv.messages = append(cs.pending, Message{Role: "assistant", Content: string(cs.text)})
	}
	return false
}

// Chunk returns the most recently read chunk.
func (cs *ConversationStream) Chunk() ChatChunk {
	return cs.inner.Chunk()
}

// Err returns the first error encountered while streaming, if any.
func (cs *ConversationStream) Err() error {
	return cs.inner.Err()
}

// Close releases the underlying connection.
func (cs *ConversationStream) Close() error {
	return cs.inner.Close()
}
