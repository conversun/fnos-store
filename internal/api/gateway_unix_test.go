package api

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestGatewayUnixHTTPStreamsBeforeCompletion(t *testing.T) {
	dir, err := os.MkdirTemp("", "gateway-uds-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	}()
	socketPath := filepath.Join(dir, "app.sock")
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{
		Mux: http.NewServeMux(), gatewayMode: true,
		staticFS: fstest.MapFS{"web/assets/test.js": {Data: []byte("window.store=true;")}},
	}
	release := make(chan struct{})
	defer close(release)
	s.Mux.HandleFunc("POST /api/stream", func(w http.ResponseWriter, r *http.Request) {
		stream, err := newSSEStream(w, r, "fixture")
		if err != nil {
			t.Error(err)
			return
		}
		if err := stream.sendProgress(progressPayload{Step: "running"}); err != nil {
			t.Error(err)
			return
		}
		select {
		case <-release:
			if err := stream.sendProgress(progressPayload{Step: "done"}); err != nil {
				t.Error(err)
			}
		case <-r.Context().Done():
		}
	})
	s.Mux.HandleFunc("/", s.handleSPA)
	server := &http.Server{Handler: s}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	defer func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, admin := range []bool{false, true} {
		req, err := http.NewRequestWithContext(t.Context(), "GET", "http://nas.test/app/fnos-apps-store/assets/test.js", nil)
		if err != nil {
			t.Fatal(err)
		}
		if admin {
			req.Header.Set("X-Trim-Userid", "1000")
			req.Header.Set("X-Trim-Isadmin", "true")
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read=%v close=%v", readErr, closeErr)
		}
		if admin && (response.StatusCode != 200 || string(body) != "window.store=true;") {
			t.Fatalf("admin asset: status=%d body=%q", response.StatusCode, body)
		}
		if !admin && response.StatusCode != 401 {
			t.Fatalf("anonymous asset status=%d", response.StatusCode)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), "POST", "http://nas.test/app/fnos-apps-store/api/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Trim-Userid", "1000")
	req.Header.Set("X-Trim-Isadmin", "true")
	req.Header.Set("Origin", "http://nas.test")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "text/event-stream" || response.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatalf("stream response: status=%d headers=%v", response.StatusCode, response.Header)
	}
	reader := bufio.NewReader(response.Body)
	event, err := reader.ReadString('\n')
	if err != nil || event != "event: progress\n" {
		t.Fatalf("first event=%q err=%v", event, err)
	}
	data, err := reader.ReadString('\n')
	if err != nil || !strings.Contains(data, `"step":"running"`) {
		t.Fatalf("first data=%q err=%v", data, err)
	}
	// The first event must arrive while the operation is still blocked.
	release <- struct{}{}
	tail, err := io.ReadAll(reader)
	if err != nil || !strings.Contains(string(tail), `"step":"done"`) {
		t.Fatalf("terminal event=%q err=%v", tail, err)
	}
}
