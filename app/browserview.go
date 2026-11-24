package main

import (
	"context"
	"errors"
	"sync"

	"github.com/elionaz/panal/internal/browser"
	"github.com/elionaz/panal/internal/protocol"
)

// The Browser tab: a live image of a session's own Chrome (package
// browser's Watch). One session is watched at a time (the tab shown);
// frames go to the frontend as events:
//
//	browser:pages:<id>  {pages, current}
//	browser:frame:<id>  {page, data}   (base64 JPEG)
//	browser:end:<id>    message        (the browser went away)

type browserView struct {
	mu      sync.Mutex
	session string
	cancel  context.CancelFunc
	pick    chan string
	gen     int
}

type viewPages struct {
	Pages   []browser.ViewPage `json:"pages"`
	Current string             `json:"current"`
}

type viewFrame struct {
	Page string `json:"page"`
	Data string `json:"data"`
}

type viewSink struct {
	a  *App
	id string
}

func (s viewSink) Pages(p []browser.ViewPage, cur string) {
	s.a.emit("browser:pages:"+s.id, viewPages{Pages: p, Current: cur})
}

func (s viewSink) Frame(page, jpeg string) {
	s.a.emit("browser:frame:"+s.id, viewFrame{Page: page, Data: jpeg})
}

// watchBrowser is package browser's Watch (tests swap it).
var watchBrowser = browser.Watch

// WatchBrowser starts showing sessionID's browser (it must be open), and
// stops showing any other.
func (a *App) WatchBrowser(sessionID string) error {
	var info protocol.BrowserInfo
	err := a.call(func(ctx context.Context) (err error) {
		info, err = a.client.Browser(ctx, protocol.MethodBrowserPrepare, sessionID)
		return err
	})
	if err != nil {
		return err
	}
	if !info.Running {
		return errors.New("this session's browser is not open")
	}
	a.startView(sessionID, info.URL)
	return nil
}

// startView watches the browser at url for sessionID, replacing the
// watched one.
func (a *App) startView(sessionID, url string) {
	v := &a.view
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cancel != nil {
		v.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	pick := make(chan string, 1)
	v.gen++
	v.session, v.cancel, v.pick = sessionID, cancel, pick
	gen := v.gen
	go func() {
		err := watchBrowser(ctx, url, pick, viewSink{a: a, id: sessionID})
		ended := ctx.Err() == nil // the browser went, not an unwatch
		v.mu.Lock()
		mine := v.gen == gen
		if mine {
			v.session, v.cancel, v.pick = "", nil, nil
		}
		v.mu.Unlock()
		cancel()
		if mine && ended {
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			a.emit("browser:end:"+sessionID, msg)
		}
	}()
}

// PickBrowserPage shows pageID of the watched session's browser.
func (a *App) PickBrowserPage(sessionID, pageID string) {
	v := &a.view
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.session != sessionID || v.pick == nil {
		return
	}
	select {
	case <-v.pick: // an earlier pick not taken yet: this one wins
	default:
	}
	v.pick <- pageID
}

// UnwatchBrowser stops showing sessionID's browser.
func (a *App) UnwatchBrowser(sessionID string) {
	v := &a.view
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.session == sessionID && v.cancel != nil {
		v.cancel()
		v.session, v.cancel, v.pick = "", nil, nil
	}
}
