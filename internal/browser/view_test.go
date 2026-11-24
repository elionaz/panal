package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// fakeDevTools is a browser's DevTools endpoint: /json/version and a
// WebSocket that answers the calls Watch makes.
type fakeDevTools struct {
	srv *httptest.Server

	mu    sync.Mutex
	conn  *websocket.Conn
	calls []string // "method target-or-session"
	acks  int
	wmu   sync.Mutex
}

func newFakeDevTools(t *testing.T, pages ...string) *fakeDevTools {
	f := &fakeDevTools{}
	up := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"webSocketDebuggerUrl":"ws://%s/devtools/browser/x"}`, r.Host)
	})
	mux.HandleFunc("/devtools/browser/x", func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.conn = c
		f.mu.Unlock()
		for {
			var m struct {
				ID        int64           `json:"id"`
				Method    string          `json:"method"`
				Params    json.RawMessage `json:"params"`
				SessionID string          `json:"sessionId"`
			}
			if c.ReadJSON(&m) != nil {
				return
			}
			var p struct {
				TargetID  string `json:"targetId"`
				SessionID any    `json:"sessionId"`
			}
			_ = json.Unmarshal(m.Params, &p)
			f.mu.Lock()
			f.calls = append(f.calls, strings.TrimSpace(m.Method+" "+p.TargetID+m.SessionID))
			if m.Method == "Page.screencastFrameAck" {
				f.acks++
			}
			f.mu.Unlock()
			result := map[string]any{}
			var after func()
			switch m.Method {
			case "Target.setDiscoverTargets":
				for _, id := range pages {
					f.page("Target.targetCreated", id, "https://x/"+id)
				}
				f.event("Target.targetCreated", map[string]any{"targetInfo": map[string]any{
					"targetId": "sw", "type": "service_worker", "url": "https://x/sw.js"}}, "")
			case "Target.attachToTarget":
				result["sessionId"] = "S-" + p.TargetID
			case "Page.captureScreenshot":
				result["data"] = "shot-" + strings.TrimPrefix(m.SessionID, "S-")
			case "Page.startScreencast":
				sess := m.SessionID
				after = func() {
					f.event("Page.screencastFrame", map[string]any{"data": "frame-" + strings.TrimPrefix(sess, "S-"),
						"sessionId": 7, "metadata": map[string]any{}}, sess)
				}
			}
			f.reply(m.ID, result)
			if after != nil {
				after()
			}
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// page sends a page target's event (id "blank" is about:blank).
func (f *fakeDevTools) page(method, id, url string) {
	if id == "blank" {
		url = "about:blank"
	}
	f.event(method, map[string]any{"targetInfo": map[string]any{"targetId": id, "type": "page", "title": "T" + id, "url": url}}, "")
}

func (f *fakeDevTools) send(v any) {
	f.wmu.Lock()
	defer f.wmu.Unlock()
	f.mu.Lock()
	c := f.conn
	f.mu.Unlock()
	_ = c.WriteJSON(v)
}

func (f *fakeDevTools) reply(id int64, result any) {
	f.send(map[string]any{"id": id, "result": result})
}

func (f *fakeDevTools) event(method string, params any, session string) {
	m := map[string]any{"method": method, "params": params}
	if session != "" {
		m["sessionId"] = session
	}
	f.send(m)
}

func (f *fakeDevTools) called(want string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == want {
			return true
		}
	}
	return false
}

type recSink struct {
	mu      sync.Mutex
	pages   []ViewPage
	current string
	frames  []string
}

func (r *recSink) Pages(p []ViewPage, cur string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pages, r.current = p, cur
}

func (r *recSink) Frame(page, jpeg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = append(r.frames, page+":"+jpeg)
}

func (r *recSink) has(frame string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.frames {
		if f == frame {
			return true
		}
	}
	return false
}

func (r *recSink) state() (int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pages), r.current
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestWatchShowsTheNewestPageAndFollowsPicks(t *testing.T) {
	f := newFakeDevTools(t, "A", "B")
	sink := &recSink{}
	pick := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Watch(ctx, f.srv.URL, pick, sink) }()

	// The newest page first: its screenshot, then the screencast's frame
	// (acknowledged). The service worker is no page.
	waitFor(t, "B's frames", func() bool { return sink.has("B:shot-B") && sink.has("B:frame-B") })
	waitFor(t, "the ack", func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.acks == 1 })
	if n, cur := sink.state(); n != 2 || cur != "B" {
		t.Fatalf("pages %d, current %q; want 2, B", n, cur)
	}

	pick <- "A"
	waitFor(t, "A's frame", func() bool { return sink.has("A:frame-A") })
	if !f.called("Target.detachFromTarget") || !f.called("Page.stopScreencast S-B") {
		t.Fatalf("B not left: %v", f.calls)
	}

	// The picked page closes: back to the newest one.
	f.event("Target.targetDestroyed", map[string]any{"targetId": "A"}, "")
	waitFor(t, "back on B", func() bool { n, cur := sink.state(); return n == 1 && cur == "B" })

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Watch after cancel: %v", err)
	}
}

// Unpicked, the view follows the agent: of the pages already open one
// not blank, then the page it opens or navigates.
func TestWatchFollowsTheAgent(t *testing.T) {
	f := newFakeDevTools(t, "X", "blank")
	sink := &recSink{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Watch(ctx, f.srv.URL, nil, sink) }()
	waitFor(t, "X, not the blank page", func() bool { _, cur := sink.state(); return cur == "X" })
	f.page("Target.targetCreated", "N", "https://x/new")
	waitFor(t, "the new page", func() bool { _, cur := sink.state(); return cur == "N" })
	f.page("Target.targetInfoChanged", "X", "https://x/other")
	waitFor(t, "the navigated page", func() bool { n, cur := sink.state(); return n == 3 && cur == "X" })
	f.page("Target.targetInfoChanged", "N", "https://x/new") // a title change only: stays
	time.Sleep(50 * time.Millisecond)
	if _, cur := sink.state(); cur != "X" {
		t.Fatalf("current %q after N's title changed; want X", cur)
	}
}

func TestWatchEndsWhenTheBrowserGoes(t *testing.T) {
	f := newFakeDevTools(t, "A")
	sink := &recSink{}
	done := make(chan error, 1)
	go func() { done <- Watch(context.Background(), f.srv.URL, nil, sink) }()
	waitFor(t, "A's frame", func() bool { return sink.has("A:frame-A") })
	f.mu.Lock()
	_ = f.conn.Close()
	f.mu.Unlock()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Watch ended without an error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch still running after the browser went")
	}
}

func TestWatchNoBrowser(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	if err := Watch(context.Background(), srv.URL, nil, &recSink{}); err == nil {
		t.Fatal("Watch with no browser: no error")
	}
}
