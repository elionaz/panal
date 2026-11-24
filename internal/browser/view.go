package browser

// The app's view of a session's Chrome: its pages and a live image of one,
// through the DevTools protocol (Target.* to follow the pages,
// Page.startScreencast for the image). It only looks: the agent keeps
// driving the browser through its own DevTools connection.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ViewPage is one of the browser's pages (tabs).
type ViewPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ViewSink receives what Watch sees. Calls come from Watch's goroutine.
type ViewSink interface {
	// Pages: the browser's pages in the order they opened, and the one
	// shown ("" none).
	Pages(pages []ViewPage, current string)
	// Frame: an image of the current page (base64 JPEG).
	Frame(page, jpeg string)
}

// Screencast settings: JPEG at this quality, at most this size (Chrome
// scales the page down to fit).
const (
	viewQuality = 70
	viewMaxW    = 1600
	viewMaxH    = 1200
	viewDialTO  = 5 * time.Second
	viewCallTO  = 10 * time.Second
)

// Watch shows the browser whose DevTools endpoint is httpURL
// (http://127.0.0.1:port) until ctx ends or the browser goes away. The
// page shown is the last one picked (pick receives page ids) while it
// exists, else the one the agent last opened or navigated (of the pages
// already open, one that is not blank).
func Watch(ctx context.Context, httpURL string, pick <-chan string, sink ViewSink) error {
	ws, err := browserWS(ctx, httpURL)
	if err != nil {
		return err
	}
	c, err := dialCDP(ctx, ws)
	if err != nil {
		return err
	}
	defer c.close()
	stop := context.AfterFunc(ctx, c.close)
	defer stop()

	v := &viewer{c: c, sink: sink, failed: map[string]bool{}}
	if _, err := v.call(ctx, "Target.setDiscoverTargets", map[string]any{"discover": true}, ""); err != nil {
		return err
	}
	// The pages already open arrive before the reply, in no useful order.
	for _, m := range c.drain() {
		v.event(m)
	}
	v.live = true
	v.settle(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.done:
			if ctx.Err() != nil {
				return nil
			}
			return c.err
		case id := <-pick:
			v.chosen = id
			delete(v.failed, id)
		case <-c.notify:
		}
		for _, m := range c.drain() {
			v.event(m)
		}
		v.settle(ctx)
	}
}

type viewer struct {
	c      *cdpConn
	sink   ViewSink
	pages  []ViewPage
	chosen string // picked by the user
	cur    string // shown
	sess   string // CDP session attached to cur
	active string // the page the agent last opened or navigated
	live   bool   // past the pages already open
	failed map[string]bool
	dirty  bool // pages or cur changed since the sink last heard
}

func (v *viewer) call(ctx context.Context, method string, params any, session string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, viewCallTO)
	defer cancel()
	return v.c.call(ctx, method, params, session)
}

type targetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

// shownPage is a target the view lists: a tab, not DevTools or an
// extension's page.
func shownPage(t targetInfo) bool {
	return t.Type == "page" && !strings.HasPrefix(t.URL, "devtools://") && !strings.HasPrefix(t.URL, "chrome-extension://")
}

func (v *viewer) event(m cdpMsg) {
	switch m.Method {
	case "Target.targetCreated", "Target.targetInfoChanged":
		var p struct {
			TargetInfo targetInfo `json:"targetInfo"`
		}
		if json.Unmarshal(m.Params, &p) != nil {
			return
		}
		t := p.TargetInfo
		i := v.index(t.TargetID)
		switch {
		case !shownPage(t) && i >= 0:
			v.remove(t.TargetID)
		case !shownPage(t):
		case i >= 0:
			if v.live && v.pages[i].URL != t.URL {
				v.active = t.TargetID
			}
			v.pages[i].Title, v.pages[i].URL = t.Title, t.URL
			delete(v.failed, t.TargetID)
			v.dirty = true
		default:
			v.pages = append(v.pages, ViewPage{ID: t.TargetID, Title: t.Title, URL: t.URL})
			if v.live {
				v.active = t.TargetID
			}
			v.dirty = true
		}
	case "Target.targetDestroyed":
		var p struct {
			TargetID string `json:"targetId"`
		}
		if json.Unmarshal(m.Params, &p) == nil {
			v.remove(p.TargetID)
		}
	case "Target.detachedFromTarget":
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if json.Unmarshal(m.Params, &p) == nil && p.SessionID != "" && p.SessionID == v.sess {
			v.sess, v.cur, v.dirty = "", "", true
		}
	case "Page.screencastFrame":
		if m.SessionID == "" || m.SessionID != v.sess {
			return
		}
		var p struct {
			Data      string `json:"data"`
			SessionID int    `json:"sessionId"`
		}
		if json.Unmarshal(m.Params, &p) != nil {
			return
		}
		// Chrome sends the next frame once this one is acknowledged.
		v.c.send("Page.screencastFrameAck", map[string]any{"sessionId": p.SessionID}, m.SessionID)
		v.sink.Frame(v.cur, p.Data)
	}
}

func (v *viewer) index(id string) int {
	for i, p := range v.pages {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (v *viewer) remove(id string) {
	i := v.index(id)
	if i < 0 {
		return
	}
	v.pages = append(v.pages[:i], v.pages[i+1:]...)
	delete(v.failed, id)
	if v.cur == id {
		v.cur, v.sess = "", "" // its session went with it
	}
	v.dirty = true
}

// want is the page to show: the picked one while it exists, else the
// agent's active one, else the newest one not blank, else any (of those
// that could be attached).
func (v *viewer) want() string {
	ok := func(id string) bool { return id != "" && v.index(id) >= 0 && !v.failed[id] }
	if ok(v.chosen) {
		return v.chosen
	}
	if ok(v.active) {
		return v.active
	}
	pick := ""
	for i := len(v.pages) - 1; i >= 0; i-- {
		p := v.pages[i]
		if v.failed[p.ID] {
			continue
		}
		if p.URL != "about:blank" && p.URL != "" {
			return p.ID
		}
		if pick == "" {
			pick = p.ID
		}
	}
	return pick
}

// settle shows the wanted page and tells the sink what changed.
func (v *viewer) settle(ctx context.Context) {
	for w := v.want(); w != v.cur && ctx.Err() == nil; w = v.want() {
		v.detach(ctx)
		if w == "" {
			break
		}
		if err := v.attach(ctx, w); err != nil {
			v.failed[w] = true
		}
	}
	if v.dirty {
		v.dirty = false
		v.sink.Pages(append([]ViewPage(nil), v.pages...), v.cur)
	}
}

func (v *viewer) detach(ctx context.Context) {
	if v.sess != "" {
		v.c.send("Page.stopScreencast", nil, v.sess)
		_, _ = v.call(ctx, "Target.detachFromTarget", map[string]any{"sessionId": v.sess}, "")
	}
	v.cur, v.sess, v.dirty = "", "", true
}

func (v *viewer) attach(ctx context.Context, id string) error {
	r, err := v.call(ctx, "Target.attachToTarget", map[string]any{"targetId": id, "flatten": true}, "")
	if err != nil {
		return err
	}
	var a struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(r, &a); err != nil || a.SessionID == "" {
		return fmt.Errorf("attach %s: no session", id)
	}
	v.cur, v.sess, v.dirty = id, a.SessionID, true
	// The screencast sends a frame when the page repaints: a still page
	// gets its first image from a screenshot.
	if r, err := v.call(ctx, "Page.captureScreenshot", map[string]any{"format": "jpeg", "quality": viewQuality}, a.SessionID); err == nil {
		var s struct {
			Data string `json:"data"`
		}
		if json.Unmarshal(r, &s) == nil && s.Data != "" {
			v.sink.Frame(id, s.Data)
		}
	}
	_, err = v.call(ctx, "Page.startScreencast", map[string]any{"format": "jpeg", "quality": viewQuality,
		"maxWidth": viewMaxW, "maxHeight": viewMaxH, "everyNthFrame": 1}, a.SessionID)
	return err
}

// browserWS is the browser's DevTools WebSocket (from /json/version).
func browserWS(ctx context.Context, httpURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, viewDialTO)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(httpURL, "/")+"/json/version", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v struct {
		WS string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", fmt.Errorf("browser version: %w", err)
	}
	if v.WS == "" {
		return "", errors.New("the browser gave no DevTools WebSocket")
	}
	return v.WS, nil
}

// cdpConn is a DevTools connection: calls wait for their reply; events
// queue (never blocking the reader, which must keep delivering replies).
type cdpConn struct {
	ws      *websocket.Conn
	wmu     sync.Mutex
	mu      sync.Mutex
	next    int64
	pending map[int64]chan cdpMsg
	queue   []cdpMsg
	notify  chan struct{} // an event was queued
	done    chan struct{} // the connection ended (err says why)
	err     error
	once    sync.Once
}

type cdpMsg struct {
	ID        int64           `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *cdpError       `json:"error,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func dialCDP(ctx context.Context, url string) (*cdpConn, error) {
	ctx, cancel := context.WithTimeout(ctx, viewDialTO)
	defer cancel()
	d := websocket.Dialer{HandshakeTimeout: viewDialTO}
	ws, _, err := d.DialContext(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("browser DevTools: %w", err)
	}
	ws.SetReadLimit(64 << 20) // frames and screenshots
	c := &cdpConn{ws: ws, pending: map[int64]chan cdpMsg{}, notify: make(chan struct{}, 1), done: make(chan struct{})}
	go c.read()
	return c, nil
}

func (c *cdpConn) read() {
	for {
		var m cdpMsg
		if err := c.ws.ReadJSON(&m); err != nil {
			c.end(err)
			return
		}
		c.mu.Lock()
		if m.ID != 0 {
			if ch := c.pending[m.ID]; ch != nil {
				delete(c.pending, m.ID)
				ch <- m
			}
		} else if m.Method != "" {
			c.queue = append(c.queue, m)
			select {
			case c.notify <- struct{}{}:
			default:
			}
		}
		c.mu.Unlock()
	}
}

func (c *cdpConn) end(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
		_ = c.ws.Close()
	})
}

func (c *cdpConn) close() { c.end(errors.New("closed")) }

func (c *cdpConn) drain() []cdpMsg {
	c.mu.Lock()
	defer c.mu.Unlock()
	q := c.queue
	c.queue = nil
	return q
}

func (c *cdpConn) write(m map[string]any) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.ws.SetWriteDeadline(time.Now().Add(viewCallTO))
	return c.ws.WriteJSON(m)
}

func message(id int64, method string, params any, session string) map[string]any {
	m := map[string]any{"id": id, "method": method}
	if params != nil {
		m["params"] = params
	}
	if session != "" {
		m["sessionId"] = session
	}
	return m
}

// send writes a call without waiting for its reply.
func (c *cdpConn) send(method string, params any, session string) {
	c.mu.Lock()
	c.next++
	id := c.next
	c.mu.Unlock()
	_ = c.write(message(id, method, params, session))
}

func (c *cdpConn) call(ctx context.Context, method string, params any, session string) (json.RawMessage, error) {
	ch := make(chan cdpMsg, 1)
	c.mu.Lock()
	c.next++
	id := c.next
	c.pending[id] = ch
	c.mu.Unlock()
	drop := func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}
	if err := c.write(message(id, method, params, session)); err != nil {
		drop()
		return nil, err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, m.Error.Message)
		}
		return m.Result, nil
	case <-c.done:
		drop()
		return nil, fmt.Errorf("%s: the browser connection ended", method)
	case <-ctx.Done():
		drop()
		return nil, fmt.Errorf("%s: %w", method, ctx.Err())
	}
}
