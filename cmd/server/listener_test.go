package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "store-uds-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(dir, "app.sock")
}

func TestStoreListenerUnixHTTP(t *testing.T) {
	path := testSocketPath(t)
	// An invalid TCP address proves Unix mode does not open or fall back to TCP.
	ln, err := listenStore("invalid TCP address", path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, r.Method+" "+r.URL.RequestURI()); err != nil {
			t.Error(err)
		}
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(ln) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("socket permissions: info=%v err=%v", info, err)
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Get("http://nas.test/app/fnos-apps-store/api/status?x=a%2Fb")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || string(body) != "GET /app/fnos-apps-store/api/status?x=a%2Fb" {
		t.Fatalf("response: status=%d body=%q err=%v", response.StatusCode, body, err)
	}
	if err := server.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket survives shutdown: %v", err)
	}
}

func TestStoreListenerTCP(t *testing.T) {
	ln, err := listenStore("127.0.0.1:0", "")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreListenerSocketConflicts(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "live", "stale", "missing parent"} {
		t.Run(kind, func(t *testing.T) {
			path := testSocketPath(t)
			switch kind {
			case "file":
				if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("untouched-target", path); err != nil {
					t.Fatal(err)
				}
			case "live", "stale":
				ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
				if err != nil {
					t.Fatal(err)
				}
				if kind == "stale" {
					ln.SetUnlinkOnClose(false)
					if err := ln.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					t.Cleanup(func() {
						if err := ln.Close(); err != nil {
							t.Error(err)
						}
					})
				}
			case "missing parent":
				path = filepath.Join(filepath.Dir(path), "missing", "app.sock")
			}
			before, statErr := os.Lstat(path)
			ln, err := listenStore("127.0.0.1:0", path)
			if kind == "stale" {
				if err != nil {
					t.Fatal(err)
				}
				if err := ln.Close(); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				if closeErr := ln.Close(); closeErr != nil {
					t.Error(closeErr)
				}
				t.Fatal("socket conflict unexpectedly accepted or fell back to TCP")
			}
			if statErr == nil {
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("existing path replaced: %v", err)
				}
			}
		})
	}
}
