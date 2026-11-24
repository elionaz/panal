package main

import (
	"context"
	"errors"
	"testing"

	"github.com/elionaz/panal/internal/browser"
)

// The Browser tab's watch: one session at a time, picks reach it, and
// the frontend hears when the browser goes (not when it unwatches).
func TestBrowserViewLifecycle(t *testing.T) {
	type run struct {
		url  string
		pick <-chan string
		ctx  context.Context
		end  chan error
	}
	runs := make(chan run, 4)
	old := watchBrowser
	watchBrowser = func(ctx context.Context, url string, pick <-chan string, sink browser.ViewSink) error {
		r := run{url, pick, ctx, make(chan error)}
		runs <- r
		sink.Pages([]browser.ViewPage{{ID: "P1"}}, "P1")
		sink.Frame("P1", "jpeg")
		select {
		case <-ctx.Done():
			return nil
		case err := <-r.end:
			return err
		}
	}
	t.Cleanup(func() { watchBrowser = old })

	var rec recorder
	a := NewApp()
	a.emit = rec.emit
	a.ctx = context.Background()

	a.startView("s1", "http://127.0.0.1:9222")
	r1 := <-runs
	waitFor(t, "s1's frame", func() bool { return rec.has("browser:frame:s1") && rec.has("browser:pages:s1") })
	a.PickBrowserPage("s1", "P2")
	if got := <-r1.pick; got != "P2" {
		t.Fatalf("pick = %q", got)
	}
	a.PickBrowserPage("other", "P3") // not watched: dropped
	select {
	case got := <-r1.pick:
		t.Fatalf("a pick for another session reached s1: %q", got)
	default:
	}

	// Watching s2 stops s1, quietly.
	a.startView("s2", "http://127.0.0.1:9223")
	r2 := <-runs
	<-r1.ctx.Done()
	if r2.url != "http://127.0.0.1:9223" {
		t.Fatalf("s2 url = %q", r2.url)
	}

	// s2's browser goes: the frontend hears it.
	r2.end <- errors.New("gone")
	waitFor(t, "s2's end", func() bool { return rec.has("browser:end:s2") })
	if rec.has("browser:end:s1") {
		t.Fatal("s1 was unwatched, not ended")
	}

	a.startView("s3", "u")
	r3 := <-runs
	a.UnwatchBrowser("s3")
	<-r3.ctx.Done()
	a.UnwatchBrowser("s3") // twice: nothing
}
