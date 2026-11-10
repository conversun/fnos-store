package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"syscall"
	"time"
)

// listenStore keeps the installed gateway transport separate from standalone TCP.
func listenStore(addr, socketPath string) (net.Listener, error) {
	if socketPath == "" {
		return net.Listen("tcp", addr)
	}
	info, err := os.Lstat(socketPath)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("socket path %s is not a socket: %w", socketPath, fs.ErrExist)
		}
		conn, dialErr := net.DialTimeout("unix", socketPath, time.Second)
		if dialErr == nil {
			return nil, errors.Join(fmt.Errorf("socket %s is already listening: %w", socketPath, fs.ErrExist), conn.Close())
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) {
			return nil, fmt.Errorf("probe socket %s: %w", socketPath, dialErr)
		}
		if err := os.Remove(socketPath); err != nil {
			return nil, fmt.Errorf("remove stale socket: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect socket: %w", err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socketPath, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("restrict socket permissions: %w", err), listener.Close())
	}
	return listener, nil
}
