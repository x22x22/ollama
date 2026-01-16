package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ollama/ollama/api"
)

// RemoteClient encapsulates client state for interacting with OpenAI-compatible APIs
type RemoteClient struct {
	base   *url.URL
	http   *http.Client
	apiKey string
}

// NewRemoteClient creates a new OpenAI-compatible client
func NewRemoteClient(base *url.URL, apiKey string, httpClient *http.Client) *RemoteClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &RemoteClient{
		base:   base,
		http:   httpClient,
		apiKey: apiKey,
	}
}

// ChatCompletion sends a chat completion request to an OpenAI-compatible API
// and converts the response back to Ollama's ChatResponse format
func (c *RemoteClient) ChatCompletion(ctx context.Context, req *api.ChatRequest, fn func(api.ChatResponse) error) error {
	// Convert Ollama ChatRequest to OpenAI ChatCompletionRequest
	openaiReq, err := c.convertToOpenAIRequest(req)
	if err != nil {
		return fmt.Errorf("failed to convert request: %w", err)
	}

	// Build the request URL
	requestURL := *c.base
	requestURL.Path = "/v1/chat/completions"

	// Marshal request body
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", requestURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	// Send request
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	// Handle streaming vs non-streaming
	stream := req.Stream != nil && *req.Stream
	if stream {
		return c.handleStreamingResponse(resp, fn, req.Model)
	}

	return c.handleNonStreamingResponse(resp, fn, req.Model)
}

func (c *RemoteClient) convertToOpenAIRequest(req *api.ChatRequest) (*ChatCompletionRequest, error) {
	// Convert messages
	var messages []Message
	for _, msg := range req.Messages {
		oaiMsg := Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			var toolCalls []ToolCall
			for i, tc := range msg.ToolCalls {
				args, err := json.Marshal(tc.Function.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal tool call arguments: %w", err)
				}
				toolCalls = append(toolCalls, ToolCall{
					ID:    tc.ID,
					Type:  "function",
					Index: i,
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      tc.Function.Name,
						Arguments: string(args),
					},
				})
			}
			oaiMsg.ToolCalls = toolCalls
		}

		// Handle tool responses
		if msg.ToolCallID != "" {
			oaiMsg.ToolCallID = msg.ToolCallID
		}

		messages = append(messages, oaiMsg)
	}

	// Build the request
	openaiReq := &ChatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream != nil && *req.Stream,
		Tools:    req.Tools,
	}

	// Map options
	if req.Options != nil {
		if temp, ok := req.Options["temperature"].(float64); ok {
			openaiReq.Temperature = &temp
		}
		if topP, ok := req.Options["top_p"].(float64); ok {
			openaiReq.TopP = &topP
		}
		if numPredict, ok := req.Options["num_predict"].(int); ok {
			openaiReq.MaxTokens = &numPredict
		}
		if seed, ok := req.Options["seed"].(int); ok {
			openaiReq.Seed = &seed
		}
	}

	// Map format (JSON schema)
	if req.Format != nil {
		openaiReq.ResponseFormat = &ResponseFormat{
			Type: "json_schema",
			JsonSchema: &JsonSchema{
				Schema: req.Format,
			},
		}
	}

	return openaiReq, nil
}

func (c *RemoteClient) handleNonStreamingResponse(resp *http.Response, fn func(api.ChatResponse) error, model string) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var openaiResp ChatCompletion
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert to Ollama format
	chatResp := c.convertFromOpenAIResponse(&openaiResp, model)
	return fn(chatResp)
}

func (c *RemoteClient) handleStreamingResponse(resp *http.Response, fn func(api.ChatResponse) error, model string) error {
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Check for SSE event format
		if len(line) < 6 || line[:6] != "data: " {
			continue
		}

		// Extract data
		data := line[6:]

		// Check for [DONE] marker
		if data == "[DONE]" {
			continue
		}

		// Parse chunk
		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // Skip malformed chunks
		}

		// Convert to Ollama format
		chatResp := c.convertFromOpenAIChunk(&chunk, model)
		if err := fn(chatResp); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

func (c *RemoteClient) convertFromOpenAIResponse(resp *ChatCompletion, model string) api.ChatResponse {
	var msg api.Message
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		msg.Role = choice.Message.Role
		msg.Content = choice.Message.Content.(string)
		
		// Convert tool calls
		if len(choice.Message.ToolCalls) > 0 {
			for _, tc := range choice.Message.ToolCalls {
				var args api.ToolCallFunctionArguments
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				msg.ToolCalls = append(msg.ToolCalls, api.ToolCall{
					ID: tc.ID,
					Function: api.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}

		// Handle reasoning/thinking if present
		if choice.Message.Reasoning != "" {
			msg.Thinking = choice.Message.Reasoning
		}
	}

	return api.ChatResponse{
		Model:     model,
		CreatedAt: time.Unix(resp.Created, 0),
		Message:   msg,
		Done:      true,
		Metrics: api.Metrics{
			PromptEvalCount: resp.Usage.PromptTokens,
			EvalCount:       resp.Usage.CompletionTokens,
		},
	}
}

func (c *RemoteClient) convertFromOpenAIChunk(chunk *ChatCompletionChunk, model string) api.ChatResponse {
	var msg api.Message
	if len(chunk.Choices) > 0 {
		choice := chunk.Choices[0]
		msg.Role = choice.Delta.Role
		msg.Content = choice.Delta.Content.(string)

		// Convert tool calls
		if len(choice.Delta.ToolCalls) > 0 {
			for _, tc := range choice.Delta.ToolCalls {
				var args api.ToolCallFunctionArguments
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				msg.ToolCalls = append(msg.ToolCalls, api.ToolCall{
					ID: tc.ID,
					Function: api.ToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}

		// Handle reasoning/thinking if present
		if choice.Delta.Reasoning != "" {
			msg.Thinking = choice.Delta.Reasoning
		}
	}

	done := false
	var doneReason string
	if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != nil {
		done = true
		doneReason = *chunk.Choices[0].FinishReason
	}

	resp := api.ChatResponse{
		Model:     model,
		CreatedAt: time.Unix(chunk.Created, 0),
		Message:   msg,
		Done:      done,
	}

	if doneReason != "" {
		resp.DoneReason = doneReason
	}

	// Add usage if present
	if chunk.Usage != nil {
		resp.Metrics.PromptEvalCount = chunk.Usage.PromptTokens
		resp.Metrics.EvalCount = chunk.Usage.CompletionTokens
	}

	return resp
}
