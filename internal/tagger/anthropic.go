package tagger

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/kite-plus/explore/internal/model"
)

// anthropicMaxTokens leaves room for models that think before they answer;
// the answer itself takes a few dozen tokens.
const anthropicMaxTokens = 4096

// anthropicAPI asks through the Anthropic Messages API, whose structured
// outputs hold the answer to the tag list.
type anthropicAPI struct {
	client anthropic.Client
	model  string
	effort string
	extra  []option.RequestOption
	schema map[string]any
}

// jsonPathEscaper keeps an extra field's name from being read as a path.
var jsonPathEscaper = strings.NewReplacer(`.`, `\.`, `*`, `\*`, `?`, `\?`)

func newAnthropic(o Options) *anthropicAPI {
	opts := []option.RequestOption{option.WithAPIKey(o.APIKey), option.WithRequestTimeout(requestTimeout)}
	if o.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(o.BaseURL))
	}
	var extra []option.RequestOption
	for k, v := range o.ExtraBody {
		extra = append(extra, option.WithJSONSet(jsonPathEscaper.Replace(k), v))
	}
	slugs := make([]string, len(model.Tags))
	for i, t := range model.Tags {
		slugs[i] = t.Slug
	}
	return &anthropicAPI{
		client: anthropic.NewClient(opts...),
		model:  o.Model,
		effort: o.Effort,
		extra:  extra,
		// The list is enforced by the schema; the limit of three is not,
		// since structured outputs take no array bounds, so parse trims.
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"tags":    map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": slugs}},
				"quality": map[string]any{"type": "string", "enum": qualities},
			},
			"required":             []string{"tags", "quality"},
			"additionalProperties": false,
		},
	}
}

func (a *anthropicAPI) ask(ctx context.Context, system, user string) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     a.model,
		MaxTokens: anthropicMaxTokens,
		System: []anthropic.TextBlockParam{{
			Text:         system,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages:     []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(user))},
		OutputConfig: anthropic.OutputConfigParam{Format: anthropic.JSONOutputFormatParam{Schema: a.schema}},
	}
	if a.effort != "" {
		params.OutputConfig.Effort = anthropic.OutputConfigEffort(a.effort)
	}
	msg, err := a.client.Messages.New(ctx, params, a.extra...)
	if err != nil {
		return "", err
	}
	switch msg.StopReason {
	case anthropic.StopReasonEndTurn:
	case anthropic.StopReasonRefusal:
		return "", errRefused
	case anthropic.StopReasonMaxTokens:
		return "", ErrTruncated
	default:
		return "", fmt.Errorf("%w: stopped at %s", ErrMalformed, msg.StopReason)
	}
	for _, block := range msg.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			return text.Text, nil
		}
	}
	return "", fmt.Errorf("%w: no text", ErrMalformed)
}
