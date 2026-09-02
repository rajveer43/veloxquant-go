package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Stream reads a Server-Sent Events chat completion stream chunk by chunk.
// Use it as:
//
//	stream, err := client.ChatCompletionStream(ctx, req)
//	for stream.Next() {
//	    chunk := stream.Chunk()
//	    fmt.Print(chunk.Choices[0].Delta.Content)
//	}
//	if err := stream.Err(); err != nil { ... }
type Stream struct {
	resp    *http.Response
	scanner *bufio.Scanner
	current StreamChunk
	err     error
	done    bool
}

// ChatCompletionStream starts a streaming chat completion request against
// POST /v1/chat/completions with stream=true.
func (c *Client) ChatCompletionStream(ctx context.Context, req ChatRequest) (*Stream, error) {
	req.Stream = true

	resp, err := c.http.Do(ctx, "POST", "/chat/completions", req)
	if err != nil {
		return nil, fmt.Errorf("chat completion stream: %w", err)
	}

	return &Stream{
		resp:    resp,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// Next advances the stream to the next chunk. It returns false when the
// stream ends, either normally or due to an error (check Err).
func (s *Stream) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	for s.scanner.Scan() {
		line := strings.TrimSpace(s.scanner.Text())
		if line == "" {
			continue
		}

		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			s.done = true
			return false
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			s.err = fmt.Errorf("decode stream chunk: %w", err)
			return false
		}

		s.current = chunk
		return true
	}

	if err := s.scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		s.err = fmt.Errorf("read stream: %w", err)
	}
	return false
}

// Chunk returns the most recently read stream chunk. Only valid after a
// call to Next that returned true.
func (s *Stream) Chunk() StreamChunk {
	return s.current
}

// Err returns the first error encountered while reading the stream, if
// any.
func (s *Stream) Err() error {
	return s.err
}

// Close releases the underlying HTTP connection. Safe to call multiple
// times.
func (s *Stream) Close() error {
	if s.resp == nil {
		return nil
	}
	return s.resp.Body.Close()
}
