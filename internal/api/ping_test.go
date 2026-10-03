package api_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// wordpressPing is what WordPress sends to each update service.
const wordpressPing = `<?xml version="1.0"?>
<methodCall>
<methodName>weblogUpdates.extendedPing</methodName>
<params>
<param><value><string>Ping &amp; Co</string></value></param>
<param><value><string>%s</string></value></param>
<param><value><string>%s</string></value></param>
</params></methodCall>`

func (e *env) dueIn(host string) time.Duration {
	e.t.Helper()
	b, err := e.s.Blog(context.Background(), host)
	if err != nil {
		e.t.Fatal(err)
	}
	return time.Until(b.NextFetchAt)
}

func (e *env) xmlrpc(body string) string {
	e.t.Helper()
	w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: body, header: map[string]string{"Content-Type": "text/xml"}})
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/xml") {
		e.t.Fatalf("XML-RPC answer = %d %s\n%s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	return w.Body.String()
}

func TestPingJSON(t *testing.T) {
	e := newEnv(t, false)
	e.seed("ping.example.com", "en")
	if d := e.dueIn("ping.example.com"); d < 50*time.Minute {
		t.Fatalf("seeded blog is due in %v", d)
	}

	w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: `{"url":"https://www.ping.example.com/posts/hello/"}`})
	if w.Code != http.StatusAccepted || w.Body.Len() != 0 {
		t.Fatalf("ping = %d %q", w.Code, w.Body.String())
	}
	// Just fetched, so the earliest is policy.PingSpacing from now.
	if d := e.dueIn("ping.example.com"); d > 6*time.Minute {
		t.Errorf("after a ping the blog is due in %v", d)
	}

	if w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: `{"url":"https://unknown.example.com/"}`}); w.Code != http.StatusAccepted {
		t.Errorf("unlisted ping = %d, want the same answer as a listed one", w.Code)
	}
	for body, want := range map[string]string{
		`{"url":"ftp://ping.example.com/"}`: "invalid_url",
		`{"url":""}`:                        "invalid_url",
		`{"url":`:                           "invalid_request",
	} {
		w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: body})
		if code, _ := errorCode(t, w); w.Code != http.StatusBadRequest || code != want {
			t.Errorf("%s: %d %s, want 400 %s", body, w.Code, code, want)
		}
	}
}

func TestPingXMLRPC(t *testing.T) {
	e := newEnv(t, false)
	e.seed("ping.example.com", "en")

	// The site address names no listed blog; the feed address does.
	got := e.xmlrpc(fmt.Sprintf(wordpressPing, "https://moved.example.org/", "https://ping.example.com/feed.xml"))
	if !strings.Contains(got, "<name>flerror</name><value><boolean>0</boolean></value>") || !strings.Contains(got, "Thanks for the ping.") {
		t.Errorf("extendedPing answer:\n%s", got)
	}
	if d := e.dueIn("ping.example.com"); d > 6*time.Minute {
		t.Errorf("the feed address did not reach the blog: due in %v", d)
	}

	plain := `<?xml version="1.0"?><methodCall><methodName>weblogUpdates.ping</methodName><params>` +
		`<param><value>Ping</value></param><param><value>https://unknown.example.com/</value></param></params></methodCall>`
	if got := e.xmlrpc(plain); !strings.Contains(got, "<boolean>0</boolean>") {
		t.Errorf("ping with untyped values:\n%s", got)
	}

	for body, fault := range map[string]string{
		`<methodCall><methodName>weblogUpdates.ping</methodName>`:                                                              "-32700",
		`<methodCall><methodName>system.listMethods</methodName><params></params></methodCall>`:                                "-32601",
		`<methodCall><methodName>weblogUpdates.ping</methodName><params><param><value>x</value></param></params></methodCall>`: "-32602",
		`<methodCall><methodName>weblogUpdates.ping</methodName><params><param><value>x</value></param>` +
			`<param><value>javascript:alert(1)</value></param></params></methodCall>`: "-32602",
	} {
		got := e.xmlrpc(body)
		if !strings.Contains(got, "<fault>") || !strings.Contains(got, "<int>"+fault+"</int>") {
			t.Errorf("%s:\n%s\nwant fault %s", body, got, fault)
		}
	}
}

func TestPingIsLimited(t *testing.T) {
	e := newEnv(t, false)
	for i := range 60 {
		if w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: `{"url":"https://a.example.com/"}`}); w.Code != http.StatusAccepted {
			t.Fatalf("ping %d = %d", i+1, w.Code)
		}
	}
	w := e.do(req{method: http.MethodPost, path: "/api/v1/ping", body: `{"url":"https://a.example.com/"}`})
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Errorf("ping 61 = %d, Retry-After %q", w.Code, w.Header().Get("Retry-After"))
	}
}
