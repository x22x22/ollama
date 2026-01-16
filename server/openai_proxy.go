package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/ollama/ollama/openai"
)

// isOpenAICompatible checks if a remote host URL is OpenAI-compatible
// It checks if the URL contains common OpenAI-compatible paths
func isOpenAICompatible(remoteHost string) bool {
	// Check for common OpenAI-compatible endpoints
	// Most OpenAI-compatible services use /v1/chat/completions
	// This includes: OpenAI, Azure OpenAI, various Chinese providers (Alibaba DashScope, etc.)
	u, err := url.Parse(remoteHost)
	if err != nil {
		return false
	}

	// Check if the path indicates it's an OpenAI-compatible endpoint
	// or if it's a base URL without /api/ which suggests OpenAI format
	path := strings.TrimSuffix(u.Path, "/")
	return !strings.HasPrefix(path, "/api") && !strings.Contains(path, "/api/")
}

// callOpenAICompatibleAPI forwards a chat request to an OpenAI-compatible endpoint
func callOpenAICompatibleAPI(ctx context.Context, remoteHost, remoteModel, apiKey string, req *api.ChatRequest, callback func(api.ChatResponse) error) error {
	// Convert api.ChatRequest to OpenAI format
	openaiReq, err := convertToOpenAIChatRequest(req, remoteModel)
	if err != nil {
		return fmt.Errorf("failed to convert request: %w", err)
	}

	// Build the full URL
	u, err := url.Parse(remoteHost)
	if err != nil {
		return fmt.Errorf("invalid remote host URL: %w", err)
	}

	// Ensure the path ends with /v1/chat/completions
	if !strings.HasSuffix(u.Path, "/v1/chat/completions") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/v1/chat/completions"
	}

	// Marshal request to JSON
	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Debug("forwarding request to OpenAI-compatible endpoint", "url", u.String(), "stream", openaiReq.Stream)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// Make the request
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Handle streaming vs non-streaming response
	if openaiReq.Stream {
		return handleOpenAIStreamingResponse(resp.Body, callback)
	}

	return handleOpenAINonStreamingResponse(resp.Body, callback)
}

// convertToOpenAIChatRequest converts api.ChatRequest to OpenAI format
func convertToOpenAIChatRequest(req *api.ChatRequest, remoteModel string) (*openai.ChatCompletionRequest, error) {
	// Convert messages
	var messages []openai.Message
	for _, msg := range req.Messages {
		openaiMsg := openai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Handle tool call ID for tool messages
		if msg.Role == "tool" {
			openaiMsg.ToolCallID = msg.ToolCallID
			if msg.ToolName != "" {
				openaiMsg.Name = msg.ToolName
			}
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			var toolCalls []openai.ToolCall
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Function.Arguments)
				toolCall := openai.ToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: string(args),
					},
				}
				toolCalls = append(toolCalls, toolCall)
			}
			openaiMsg.ToolCalls = toolCalls
		}

		// Handle images
		if len(msg.Images) > 0 {
			var contentParts []any
			if msg.Content != "" {
				contentParts = append(contentParts, map[string]any{
					"type": "text",
					"text": msg.Content,
				})
			}
			for _, img := range msg.Images {
				contentParts = append(contentParts, map[string]any{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:image/jpeg;base64," + string(img),
					},
				})
			}
			openaiMsg.Content = contentParts
		}

		// Handle thinking/reasoning
		if msg.Thinking != "" {
			openaiMsg.Reasoning = msg.Thinking
		}

		messages = append(messages, openaiMsg)
	}

	// Build options
	var temperature *float64
	var topP *float64
	var maxTokens *int
	var seed *int
	var frequencyPenalty *float64
	var presencePenalty *float64

	if req.Options != nil {
		if temp, ok := req.Options["temperature"].(float64); ok {
			temperature = &temp
		}
		if tp, ok := req.Options["top_p"].(float64); ok {
			topP = &tp
		}
		if np, ok := req.Options["num_predict"].(int); ok {
			maxTokens = &np
		}
		if s, ok := req.Options["seed"].(int); ok {
			seed = &s
		}
		if fp, ok := req.Options["frequency_penalty"].(float64); ok {
			frequencyPenalty = &fp
		}
		if pp, ok := req.Options["presence_penalty"].(float64); ok {
			presencePenalty = &pp
		}
	}

	// Handle reasoning effort
	var reasoning *openai.Reasoning
	if req.Think != nil {
		if req.Think.IsString() {
			reasoning = &openai.Reasoning{
				Effort: req.Think.String(),
			}
		}
	}

	stream := req.Stream != nil && *req.Stream

	return &openai.ChatCompletionRequest{
		Model:            remoteModel,
		Messages:         messages,
		Stream:           stream,
		MaxTokens:        maxTokens,
		Seed:             seed,
		Temperature:      temperature,
		FrequencyPenalty: frequencyPenalty,
		PresencePenalty:  presencePenalty,
		TopP:             topP,
		Tools:            req.Tools,
		Reasoning:        reasoning,
	}, nil
}

// handleOpenAIStreamingResponse processes a streaming response from OpenAI
func handleOpenAIStreamingResponse(body io.Reader, callback func(api.ChatResponse) error) error {
	scanner := bufio.NewScanner(body)
	var fullContent strings.Builder
	var fullThinking strings.Builder
	var toolCalls []api.ToolCall

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Remove "data: " prefix
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")

		// Check for [DONE] marker
		if data == "[DONE]" {
			// Send final response with done=true
			return callback(api.ChatResponse{
				Model:     "",
				CreatedAt: time.Now(),
				Message: api.Message{
					Role:      "assistant",
					Content:   fullContent.String(),
					Thinking:  fullThinking.String(),
					ToolCalls: toolCalls,
				},
				Done:       true,
				DoneReason: "stop",
			})
		}

		// Parse the chunk
		var chunk openai.ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			slog.Warn("failed to parse streaming chunk", "error", err, "data", data)
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Accumulate content
		if choice.Delta.Content != nil {
			if content, ok := choice.Delta.Content.(string); ok && content != "" {
				fullContent.WriteString(content)

				// Send intermediate response
				err := callback(api.ChatResponse{
					Model:     chunk.Model,
					CreatedAt: time.Now(),
					Message: api.Message{
						Role:    "assistant",
						Content: content,
					},
					Done: false,
				})
				if err != nil {
					return err
				}
			}
		}

		// Accumulate thinking/reasoning
		if choice.Delta.Reasoning != "" {
			fullThinking.WriteString(choice.Delta.Reasoning)

			// Send thinking update
			err := callback(api.ChatResponse{
				Model:     chunk.Model,
				CreatedAt: time.Now(),
				Message: api.Message{
					Role:     "assistant",
					Thinking: choice.Delta.Reasoning,
				},
				Done: false,
			})
			if err != nil {
				return err
			}
		}

		// Handle tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				// Check if this is a new tool call or an update to existing one
				if tc.Index < len(toolCalls) {
					// Update existing tool call
					if tc.Function.Name != "" {
						toolCalls[tc.Index].Function.Name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						var args api.ToolCallFunctionArguments
						if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
							toolCalls[tc.Index].Function.Arguments = args
						}
					}
				} else {
					// New tool call
					var args api.ToolCallFunctionArguments
					if tc.Function.Arguments != "" {
						json.Unmarshal([]byte(tc.Function.Arguments), &args)
					}
					toolCalls = append(toolCalls, api.ToolCall{
						ID: tc.ID,
						Function: api.ToolCallFunction{
							Name:      tc.Function.Name,
							Arguments: args,
						},
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// handleOpenAINonStreamingResponse processes a non-streaming response from OpenAI
func handleOpenAINonStreamingResponse(body io.Reader, callback func(api.ChatResponse) error) error {
	var completion openai.ChatCompletion
	if err := json.NewDecoder(body).Decode(&completion); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if len(completion.Choices) == 0 {
		return fmt.Errorf("no choices in response")
	}

	choice := completion.Choices[0]

	// Convert tool calls
	var toolCalls []api.ToolCall
	for _, tc := range choice.Message.ToolCalls {
		var args api.ToolCallFunctionArguments
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			slog.Warn("failed to parse tool call arguments", "error", err)
			continue
		}
		toolCalls = append(toolCalls, api.ToolCall{
			ID: tc.ID,
			Function: api.ToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: args,
			},
		})
	}

	// Extract content
	content := ""
	if c, ok := choice.Message.Content.(string); ok {
		content = c
	}

	doneReason := "stop"
	if choice.FinishReason != nil {
		doneReason = *choice.FinishReason
	}

	return callback(api.ChatResponse{
		Model:     completion.Model,
		CreatedAt: time.Unix(completion.Created, 0),
		Message: api.Message{
			Role:      choice.Message.Role,
			Content:   content,
			Thinking:  choice.Message.Reasoning,
			ToolCalls: toolCalls,
		},
		Done:            true,
		DoneReason:      doneReason,
		PromptEvalCount: completion.Usage.PromptTokens,
		EvalCount:       completion.Usage.CompletionTokens,
	})
}
