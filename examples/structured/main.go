// Example: requesting structured (JSON) output from a chat completion.
//
// The VeloxQuant runtime serves completions via mlx_lm.server, which does
// not currently enforce response_format server-side (see the ResponseFormat
// doc comment in types.go). So this example does the two things that
// actually make structured extraction reliable today:
//
//  1. Sets ResponseFormat anyway, so the request is correct and forward
//     compatible with runtimes that do honor it.
//  2. Asks for the JSON shape explicitly in the prompt, and validates the
//     model's output against the same schema before trusting it.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

type extractedContact struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	ctx := context.Background()

	client, err := veloxquant.NewClient(
		veloxquant.WithAutoDetect(),
	)
	if err != nil {
		panic(err)
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"email": map[string]any{"type": "string"},
		},
		"required":             []string{"name", "email"},
		"additionalProperties": false,
	}

	response, err := client.Chat(ctx, veloxquant.ChatRequest{
		Model: "mlx-community/Qwen3-8B-4bit",
		Messages: []veloxquant.Message{
			{
				Role: "system",
				Content: "Extract the contact from the user's message as JSON " +
					"matching this schema, with no extra text: " + mustJSON(schema),
			},
			{
				Role:    "user",
				Content: "Reach out to Priya Shah at priya.shah@example.com about the release.",
			},
		},
		ResponseFormat: veloxquant.JSONSchema("contact", schema, true),
	})
	if err != nil {
		panic(err)
	}

	var contact extractedContact
	if err := json.Unmarshal([]byte(response.Text), &contact); err != nil {
		panic(fmt.Errorf("model did not return valid JSON for the schema: %w", err))
	}

	fmt.Printf("%s <%s>\n", contact.Name, contact.Email)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
