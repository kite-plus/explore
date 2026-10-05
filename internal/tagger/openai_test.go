package tagger

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kite-plus/explore/internal/model"
)

// fakeChat answers /v1/chat/completions like an OpenAI-compatible server.
// It serves statuses in turn before answering, and keeps the last request.
type fakeChat struct {
	srv *httptest.Server

	mu       sync.Mutex
	statuses []int
	errMsg   string
	finish   string
	content  string
	refusal  string
	calls    int
	auth     string
	last     map[string]any
}

func newFakeChat(t *testing.T) *fakeChat {
	f := &fakeChat{finish: "stop"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		f.calls++
		f.auth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		f.last = map[string]any{}
		if err := json.Unmarshal(body, &f.last); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if len(f.statuses) > 0 {
			status := f.statuses[0]
			f.statuses = f.statuses[1:]
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"error":{"message":%q}}`, f.errMsg)
			return
		}
		message := map[string]any{"role": "assistant", "content": f.content}
		if f.refusal != "" {
			message["refusal"] = f.refusal
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-1", "object": "chat.completion",
			"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": f.finish}},
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeChat) tagger(key, effort string) *Tagger {
	t := New(Options{Provider: OpenAI, BaseURL: f.srv.URL + "/v1/", APIKey: key, Model: "chat-test", Effort: effort})
	t.api.(*openAIAPI).retryWait = time.Millisecond
	return t
}

func TestOpenAISendsAJSONModeRequest(t *testing.T) {
	f := newFakeChat(t)
	f.content = `{"tags":["ai","tools"],"quality":"standout"}`
	rating, err := f.tagger("sk-test", "low").Tag(t.Context(), post)
	if err != nil || !slices.Equal(rating.Tags, []string{"ai", "tools"}) || rating.Quality != model.QualityStandout {
		t.Fatalf("rating = %+v, %v", rating, err)
	}
	if f.auth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", f.auth)
	}
	req := f.last
	if req["model"] != "chat-test" || req["reasoning_effort"] != "low" {
		t.Errorf("model = %v, reasoning_effort = %v", req["model"], req["reasoning_effort"])
	}
	if format := req["response_format"].(map[string]any); format["type"] != "json_object" {
		t.Errorf("response_format = %v", format)
	}
	messages := req["messages"].([]any)
	system := messages[0].(map[string]any)
	user := messages[1].(map[string]any)
	if system["role"] != "system" || !strings.Contains(system["content"].(string), "- ai: ") || !strings.Contains(system["content"].(string), "JSON") {
		t.Errorf("system message = %v", system)
	}
	if user["role"] != "user" || !strings.Contains(user["content"].(string), "Title: "+post.Title) {
		t.Errorf("user message = %v", user)
	}
}

func TestOpenAIWithoutKeyOrEffort(t *testing.T) {
	f := newFakeChat(t)
	f.content = `{"tags":[],"quality":"brief"}`
	if _, err := f.tagger("", "").Tag(t.Context(), post); err != nil {
		t.Fatal(err)
	}
	if f.auth != "" {
		t.Errorf("Authorization sent without a key: %q", f.auth)
	}
	if _, ok := f.last["reasoning_effort"]; ok {
		t.Error("reasoning_effort sent although none was configured")
	}
}

func TestOpenAIExtraBody(t *testing.T) {
	f := newFakeChat(t)
	f.content = `{"tags":[],"quality":"brief"}`
	tg := New(Options{
		Provider: OpenAI, BaseURL: f.srv.URL + "/v1", APIKey: "k", Model: "deepseek-flash", Effort: "high",
		ExtraBody: map[string]any{"thinking": map[string]any{"type": "disabled"}, "reasoning_effort": "low"},
	})
	if _, err := tg.Tag(t.Context(), post); err != nil {
		t.Fatal(err)
	}
	if thinking, _ := f.last["thinking"].(map[string]any); thinking["type"] != "disabled" {
		t.Errorf("thinking = %v", f.last["thinking"])
	}
	if f.last["reasoning_effort"] != "low" || f.last["model"] != "deepseek-flash" {
		t.Errorf("an extra field must win over Explore's own: %v", f.last)
	}
}

func TestOpenAIAnswers(t *testing.T) {
	f := newFakeChat(t)
	f.content = "```json\n{\"tags\":[\"backend\"],\"quality\":\"solid\"}\n```"
	if rating, err := f.tagger("k", "").Tag(t.Context(), post); err != nil || !slices.Equal(rating.Tags, []string{"backend"}) || rating.Quality != model.QualitySolid {
		t.Errorf("fenced answer: %+v, %v", rating, err)
	}
	f.content, f.refusal = "", "I can't help with that."
	if rating, err := f.tagger("k", "").Tag(t.Context(), post); err != nil || len(rating.Tags) != 0 || rating.Quality != model.QualitySkip {
		t.Errorf("refusal: %+v, %v; want no tags, a skip and no error", rating, err)
	}
	f.refusal, f.finish = "", "content_filter"
	if rating, err := f.tagger("k", "").Tag(t.Context(), post); err != nil || rating.Quality != model.QualitySkip {
		t.Errorf("content filter: %+v, %v; want a skip", rating, err)
	}
	f.content, f.finish = `{"tags":["ai"`, "length"
	if _, err := f.tagger("k", "").Tag(t.Context(), post); !errors.Is(err, ErrTruncated) {
		t.Errorf("length: err = %v, want ErrTruncated", err)
	}
	f.content, f.finish = "", "stop"
	if _, err := f.tagger("k", "").Tag(t.Context(), post); !errors.Is(err, ErrMalformed) {
		t.Errorf("empty answer: err = %v, want ErrMalformed", err)
	}
}

func TestOpenAIRetries(t *testing.T) {
	f := newFakeChat(t)
	f.content = `{"tags":["life"],"quality":"solid"}`
	f.statuses = []int{http.StatusInternalServerError, http.StatusTooManyRequests}
	if rating, err := f.tagger("k", "").Tag(t.Context(), post); err != nil || rating.Quality != model.QualitySolid || f.calls != 3 {
		t.Errorf("after a server error and a rate limit: %+v, %v, %d calls", rating, err, f.calls)
	}

	f.calls, f.statuses, f.errMsg = 0, []int{http.StatusBadRequest}, "Content Exists Risk"
	_, err := f.tagger("k", "").Tag(t.Context(), post)
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "Content Exists Risk") || f.calls != 1 {
		t.Errorf("a rejected request: %v after %d calls, want the reason and no retry", err, f.calls)
	}

	f.calls, f.statuses = 0, []int{http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway, http.StatusBadGateway}
	if _, err := f.tagger("k", "").Tag(t.Context(), post); err == nil || f.calls != openAIRetries+1 {
		t.Errorf("an outage: %v after %d calls", err, f.calls)
	}
}

func TestOpenAIDefaultBaseURL(t *testing.T) {
	if got := newOpenAI(Options{}).url; got != "https://api.openai.com/v1/chat/completions" {
		t.Errorf("url = %q", got)
	}
}
