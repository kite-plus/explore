package tagger

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"
)

const (
	openAIBaseURL = "https://api.openai.com/v1"
	// openAIRetries is how often a request is tried again after a rate
	// limit, a server error or a broken connection, as the Anthropic SDK does.
	openAIRetries  = 2
	maxAnswerBytes = 1 << 20
)

// openAIAPI asks through an OpenAI-compatible chat completions API in JSON
// mode. Nearly every compatible server supports JSON mode, unlike JSON
// schemas, so parse alone holds the answer to the tag list.
type openAIAPI struct {
	client    *http.Client
	url       string
	key       string
	model     string
	effort    string
	extra     map[string]any
	retryWait time.Duration // times the attempt number
}

func newOpenAI(o Options) *openAIAPI {
	return &openAIAPI{
		client:    &http.Client{},
		url:       strings.TrimRight(cmp.Or(o.BaseURL, openAIBaseURL), "/") + "/chat/completions",
		key:       o.APIKey,
		model:     o.Model,
		effort:    o.Effort,
		extra:     o.ExtraBody,
		retryWait: time.Second,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
			Refusal string `json:"refusal"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *openAIAPI) ask(ctx context.Context, system, user string) (string, error) {
	req := map[string]any{
		"model":           a.model,
		"messages":        []chatMessage{{Role: "system", Content: system}, {Role: "user", Content: user}},
		"response_format": map[string]string{"type": "json_object"},
	}
	if a.effort != "" {
		req["reasoning_effort"] = a.effort
	}
	maps.Copy(req, a.extra)
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	var resp chatResponse
	for attempt := range openAIRetries + 1 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", err
			case <-time.After(time.Duration(attempt) * a.retryWait):
			}
		}
		var retry bool
		if resp, retry, err = a.post(ctx, body); err == nil || !retry {
			break
		}
	}
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("%w: no choices", ErrMalformed)
	}
	choice := resp.Choices[0]
	switch {
	case choice.Message.Refusal != "" || choice.FinishReason == "content_filter":
		return "", errRefused
	case choice.FinishReason == "length":
		return "", ErrTruncated
	}
	return choice.Message.Content, nil
}

// post sends one request and reports whether its failure is worth a retry.
func (a *openAIAPI) post(ctx context.Context, body []byte) (chatResponse, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.key != "" {
		req.Header.Set("Authorization", "Bearer "+a.key)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return chatResponse{}, ctx.Err() == nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerBytes))
	if err != nil {
		return chatResponse{}, ctx.Err() == nil, err
	}
	var out chatResponse
	decodeErr := json.Unmarshal(data, &out)
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
		msg := []rune(strings.TrimSpace(out.Error.Message))
		if len(msg) > 200 {
			msg = append(msg[:200], '…')
		}
		if len(msg) > 0 {
			return chatResponse{}, retry, fmt.Errorf("chat completions: %s: %s", resp.Status, string(msg))
		}
		return chatResponse{}, retry, fmt.Errorf("chat completions: %s", resp.Status)
	}
	if decodeErr != nil {
		return chatResponse{}, false, fmt.Errorf("%w: %w", ErrMalformed, decodeErr)
	}
	return out, false, nil
}
