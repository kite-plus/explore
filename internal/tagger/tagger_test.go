package tagger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/kite-plus/explore/internal/model"
)

// fakeAPI answers /v1/messages with the given stop reason and text, and
// keeps the last request body.
type fakeAPI struct {
	srv        *httptest.Server
	stopReason string
	text       string
	status     int
	last       map[string]any
}

func newFakeAPI(t *testing.T) *fakeAPI {
	f := &fakeAPI{stopReason: "end_turn", status: http.StatusOK}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("request %s with key %q", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		body, _ := io.ReadAll(r.Body)
		f.last = map[string]any{}
		if err := json.Unmarshal(body, &f.last); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"invalid_request_error","message":"no"}}`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "msg_1", "type": "message", "role": "assistant", "model": "test-model",
			"content":     []map[string]any{{"type": "text", "text": f.text}},
			"stop_reason": f.stopReason, "stop_sequence": nil,
			"usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeAPI) tagger(effort string) *Tagger {
	return New(Options{APIKey: "test-key", Model: "test-model", Effort: effort, BaseURL: f.srv.URL})
}

var post = Post{
	Title:      "给 Twikoo 接入 Jev，用 AI 判断博客评论是不是广告",
	Excerpt:    "前言 最近 Jev 火了。",
	Categories: []string{"AI", "博客"},
	Language:   "zh-CN",
	BlogTags:   []string{"tools"},
}

func TestTagSendsOneCachedPromptAndASchema(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":["ai","tools"],"quality":"solid"}`
	rating, err := f.tagger("low").Tag(context.Background(), post)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rating.Tags, []string{"ai", "tools"}) || rating.Quality != model.QualitySolid {
		t.Errorf("rating = %+v", rating)
	}

	req := f.last
	if req["model"] != "test-model" || req["max_tokens"] != float64(anthropicMaxTokens) {
		t.Errorf("model = %v, max_tokens = %v", req["model"], req["max_tokens"])
	}
	system := req["system"].([]any)[0].(map[string]any)
	if system["cache_control"].(map[string]any)["type"] != "ephemeral" {
		t.Errorf("system prompt is not cached: %v", system)
	}
	for _, tag := range model.Tags {
		if !strings.Contains(system["text"].(string), "- "+tag.Slug+": ") {
			t.Errorf("system prompt lacks %s", tag.Slug)
		}
	}
	config := req["output_config"].(map[string]any)
	if config["effort"] != "low" {
		t.Errorf("effort = %v", config["effort"])
	}
	format := config["format"].(map[string]any)
	properties := format["schema"].(map[string]any)["properties"].(map[string]any)
	enum := properties["tags"].(map[string]any)["items"].(map[string]any)["enum"].([]any)
	if format["type"] != "json_schema" || len(enum) != len(model.Tags) {
		t.Errorf("format = %v", format)
	}
	if got := properties["quality"].(map[string]any)["enum"].([]any); len(got) != 4 || got[0] != "skip" || got[3] != "standout" {
		t.Errorf("quality enum = %v", got)
	}
	if !strings.Contains(system["text"].(string), "- standout: ") {
		t.Error("system prompt does not explain the ratings")
	}
	user := req["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"Title: " + post.Title, "Categories: AI, 博客", "Language: zh-CN", "Blog's usual tags: tools"} {
		if !strings.Contains(user, want) {
			t.Errorf("message lacks %q:\n%s", want, user)
		}
	}
}

func TestTagExtraBody(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":[],"quality":"brief"}`
	tg := New(Options{APIKey: "test-key", Model: "test-model", BaseURL: f.srv.URL, ExtraBody: map[string]any{
		"reasoning": map[string]any{"effort": "none"}, "odd.name": true,
	}})
	if _, err := tg.Tag(t.Context(), post); err != nil {
		t.Fatal(err)
	}
	if reasoning, _ := f.last["reasoning"].(map[string]any); reasoning["effort"] != "none" || f.last["odd.name"] != true {
		t.Errorf("request = %v", f.last)
	}
}

func TestTagWithoutEffort(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":[],"quality":"brief"}`
	if _, err := f.tagger("").Tag(context.Background(), post); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.last["output_config"].(map[string]any)["effort"]; ok {
		t.Error("effort sent although none was configured")
	}
}

func TestTagKeepsOnlyKnownTagsAndRatings(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":["ai","nonsense","ai","backend","ops","data"],"quality":"standout"}`
	rating, err := f.tagger("").Tag(context.Background(), post)
	if err != nil || !slices.Equal(rating.Tags, []string{"ai", "backend", "ops"}) || rating.Quality != model.QualityStandout {
		t.Errorf("rating = %+v, %v", rating, err)
	}
	f.text = `{"tags":["ai"],"quality":"brilliant"}`
	if rating, err := f.tagger("").Tag(context.Background(), post); err != nil || rating.Quality != model.QualitySkip {
		t.Errorf("an unknown rating = %+v, %v; want a skip", rating, err)
	}
}

func TestTagUnusableAnswers(t *testing.T) {
	f := newFakeAPI(t)
	f.stopReason, f.text = "refusal", `{"tags":["ai"`
	rating, err := f.tagger("").Tag(t.Context(), post)
	if err != nil || len(rating.Tags) != 0 || rating.Quality != model.QualitySkip {
		t.Errorf("refusal: rating = %+v, %v; want no tags, a skip and no error", rating, err)
	}
	// A cut-short answer is tried again later rather than taken as a skip.
	f.stopReason = "max_tokens"
	if _, err := f.tagger("").Tag(t.Context(), post); !errors.Is(err, ErrTruncated) {
		t.Errorf("max_tokens: err = %v, want ErrTruncated", err)
	}
	f.stopReason, f.text = "end_turn", "not json"
	if _, err := f.tagger("").Tag(t.Context(), post); !errors.Is(err, ErrMalformed) {
		t.Errorf("err = %v, want ErrMalformed", err)
	}
}

func TestTagAPIError(t *testing.T) {
	f := newFakeAPI(t)
	f.status = http.StatusBadRequest
	if _, err := f.tagger("").Tag(context.Background(), post); err == nil {
		t.Fatal("want an error")
	}
}
