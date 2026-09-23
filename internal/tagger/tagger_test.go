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
	f.text = `{"tags":["ai","tools"]}`
	tags, err := f.tagger("low").Tag(context.Background(), post)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tags, []string{"ai", "tools"}) {
		t.Errorf("tags = %v", tags)
	}

	req := f.last
	if req["model"] != "test-model" {
		t.Errorf("model = %v", req["model"])
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
	enum := format["schema"].(map[string]any)["properties"].(map[string]any)["tags"].(map[string]any)["items"].(map[string]any)["enum"].([]any)
	if format["type"] != "json_schema" || len(enum) != len(model.Tags) {
		t.Errorf("format = %v", format)
	}
	user := req["messages"].([]any)[0].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	for _, want := range []string{"Title: " + post.Title, "Categories: AI, 博客", "Language: zh-CN", "Blog's usual tags: tools"} {
		if !strings.Contains(user, want) {
			t.Errorf("message lacks %q:\n%s", want, user)
		}
	}
}

func TestTagWithoutEffort(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":[]}`
	if _, err := f.tagger("").Tag(context.Background(), post); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.last["output_config"].(map[string]any)["effort"]; ok {
		t.Error("effort sent although none was configured")
	}
}

func TestTagKeepsOnlyKnownTags(t *testing.T) {
	f := newFakeAPI(t)
	f.text = `{"tags":["ai","nonsense","ai","backend","ops","data"]}`
	tags, err := f.tagger("").Tag(context.Background(), post)
	if err != nil || !slices.Equal(tags, []string{"ai", "backend", "ops"}) {
		t.Errorf("tags = %v, %v", tags, err)
	}
}

func TestTagUnusableAnswers(t *testing.T) {
	for _, reason := range []string{"refusal", "max_tokens"} {
		f := newFakeAPI(t)
		f.stopReason, f.text = reason, `{"tags":["ai"`
		tags, err := f.tagger("").Tag(context.Background(), post)
		if err != nil || len(tags) != 0 {
			t.Errorf("%s: tags = %v, %v; want none and no error", reason, tags, err)
		}
	}
	f := newFakeAPI(t)
	f.text = "not json"
	if _, err := f.tagger("").Tag(context.Background(), post); !errors.Is(err, ErrMalformed) {
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
