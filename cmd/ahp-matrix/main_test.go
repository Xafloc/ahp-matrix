package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer that is safe to write and read concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestRunServesTheAPIUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() { done <- run(ctx, []string{"--port", "0", "--no-open", "--data-dir", t.TempDir()}, out) }()

	urlRe := regexp.MustCompile(`open:\s+(http://\S+)`)
	var url string
	for deadline := time.Now().Add(5 * time.Second); url == "" && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if m := urlRe.FindStringSubmatch(out.String()); m != nil {
			url = m[1]
		}
	}
	if url == "" {
		t.Fatalf("server never printed its URL; output: %q", out.String())
	}

	resp, err := http.Get(url + "/api/version")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/version = %d, want 200", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run returned %v, want nil after shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not return after cancel")
	}
}

func TestListenFallsBackToAFreePort(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	busyPort := busy.Addr().(*net.TCPAddr).Port

	ln, err := listen("127.0.0.1", busyPort)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if got := ln.Addr().(*net.TCPAddr).Port; got == busyPort {
		t.Errorf("listen reused busy port %s", strconv.Itoa(got))
	}
}

func TestWarnsWhenNotLoopback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := &syncBuffer{}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"--host", "0.0.0.0", "--port", "0", "--no-open", "--data-dir", t.TempDir()}, out)
	}()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if regexp.MustCompile(`open:`).MatchString(out.String()) {
			break
		}
	}
	cancel()
	<-done
	if !regexp.MustCompile(`warning:.*no login`).MatchString(out.String()) {
		t.Errorf("no warning in output: %q", out.String())
	}
}
