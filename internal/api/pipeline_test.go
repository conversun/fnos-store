package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fnos-store/internal/platform"
)

// stubAppCenter is a scripted platform.AppCenter for verifyInstalled tests.
// Only Check and List participate in the assertions; the other methods are
// no-ops satisfying the interface.
type stubAppCenter struct {
	checkScript []stubCheckResult
	listResult  []platform.InstalledApp
	listErr     error

	appVolIdx   int
	appVolFound bool
	appVolErr   error
	volumes     []platform.VolumeInfo

	setVolCalls []int
	setVolErr   error

	nCheck int32
	nList  int32
}

type stubCheckResult struct {
	installed bool
	err       error
}

func (s *stubAppCenter) Check(appname string) (bool, error) {
	idx := int(atomic.AddInt32(&s.nCheck, 1)) - 1
	if idx >= len(s.checkScript) {
		// Script exhausted: repeat the last entry so long loops don't panic.
		idx = len(s.checkScript) - 1
	}
	r := s.checkScript[idx]
	return r.installed, r.err
}

func (s *stubAppCenter) List() ([]platform.InstalledApp, error) {
	atomic.AddInt32(&s.nList, 1)
	return s.listResult, s.listErr
}

func (s *stubAppCenter) Status(string) (string, error)               { return "", nil }
func (s *stubAppCenter) InstallFpk(string, int) error                { return nil }
func (s *stubAppCenter) InstallLocal(string, int, bool) error        { return nil }
func (s *stubAppCenter) Uninstall(string) error                      { return nil }
func (s *stubAppCenter) Start(string) error                          { return nil }
func (s *stubAppCenter) Stop(string) error                           { return nil }
func (s *stubAppCenter) DefaultVolume() (int, error)                 { return 1, nil }
func (s *stubAppCenter) ListVolumes() ([]platform.VolumeInfo, error) { return s.volumes, nil }
func (s *stubAppCenter) AppInstallVolume(string) (int, bool, error) {
	return s.appVolIdx, s.appVolFound, s.appVolErr
}
func (s *stubAppCenter) SetDefaultVolume(v int) error {
	s.setVolCalls = append(s.setVolCalls, v)
	return s.setVolErr
}

// TestVerifyInstalled locks in the retry + List() fallback contract for
// GitHub issue conversun/fnos-apps#181. Cases cover the happy path, the
// race-recovery path that motivates the fix, the List() fallback (accept
// only running/stopped, reject unknown), a hard CLI error that MUST NOT
// retry, and ctx cancellation.
func TestVerifyInstalled(t *testing.T) {
	const appName = "plexmediaserver"

	// Skip real sleeps but keep ctx-cancellation semantics.
	origWait := verifyWait
	verifyWait = func(ctx context.Context, _ time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t.Cleanup(func() { verifyWait = origWait })

	cases := []struct {
		name        string
		checkScript []stubCheckResult
		list        []platform.InstalledApp
		listErr     error
		manifest    bool
		preCancel   bool
		wantErr     bool
		wantErrSub  string
		wantCtxErr  bool
		wantNCheck  int32
		wantNList   int32
	}{
		{
			name:        "happy_first_try",
			checkScript: []stubCheckResult{{installed: true}},
			wantNCheck:  1,
			wantNList:   0,
		},
		{
			name: "race_recovers_at_4",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false},
				{installed: true},
			},
			wantNCheck: 4,
			wantNList:  0,
		},
		{
			name:        "extended_budget_attempts",
			checkScript: []stubCheckResult{{installed: false}},
			wantErr:     true,
			wantNCheck:  12,
			wantNList:   1,
		},
		{
			name: "list_fallback_hit_running",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
			},
			list:       []platform.InstalledApp{{AppName: "plexmediaserver", Status: "running"}},
			wantNCheck: 12,
			wantNList:  1,
		},
		{
			name: "list_fallback_hit_stopped",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
			},
			list:       []platform.InstalledApp{{AppName: "plexmediaserver", Status: "stopped"}},
			wantNCheck: 12,
			wantNList:  1,
		},
		{
			name:        "fs_fallback_hit_after_check_and_list_miss",
			checkScript: []stubCheckResult{{installed: false}},
			manifest:    true,
			wantNCheck:  12,
			wantNList:   1,
		},
		{
			name: "list_fallback_reject_unknown_status",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
			},
			list:       []platform.InstalledApp{{AppName: "plexmediaserver", Status: "unknown"}},
			wantErr:    true,
			wantErrSub: "验证失败",
			wantNCheck: 12,
			wantNList:  1,
		},
		{
			name: "list_fallback_wrong_appname",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
			},
			list:       []platform.InstalledApp{{AppName: "other-app", Status: "running"}},
			wantErr:    true,
			wantErrSub: "验证失败",
			wantNCheck: 12,
			wantNList:  1,
		},
		{
			name: "list_fallback_miss_empty",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
				{installed: false}, {installed: false}, {installed: false}, {installed: false},
			},
			list:       nil,
			wantErr:    true,
			wantErrSub: "验证失败",
			wantNCheck: 12,
			wantNList:  1,
		},
		{
			name:        "fs_fallback_miss",
			checkScript: []stubCheckResult{{installed: false}},
			wantErr:     true,
			wantErrSub:  "重试 12 次共 51.5s",
			wantNCheck:  12,
			wantNList:   1,
		},
		{
			name:        "hard_error_but_manifest_present_succeeds",
			checkScript: []stubCheckResult{{installed: false, err: errors.New("cli exit 2")}},
			manifest:    true,
			wantNCheck:  1,
			wantNList:   0,
		},
		{
			name:        "hard_error_no_manifest_fails_fast",
			checkScript: []stubCheckResult{{installed: false, err: errors.New("cli exit 2")}},
			wantErr:     true,
			wantErrSub:  "cli exit 2",
			wantNCheck:  1,
			wantNList:   0,
		},
		{
			name: "ctx_already_canceled",
			checkScript: []stubCheckResult{
				{installed: false}, {installed: false},
			},
			preCancel:  true,
			wantErr:    true,
			wantCtxErr: true,
			wantNCheck: 0,
			wantNList:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appsDir := t.TempDir()
			if tc.manifest {
				appDir := filepath.Join(appsDir, appName)
				if err := os.MkdirAll(appDir, 0o755); err != nil {
					t.Fatalf("create app dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(appDir, "manifest"), []byte("appname = "+appName+"\n"), 0o600); err != nil {
					t.Fatalf("create manifest: %v", err)
				}
			}

			stub := &stubAppCenter{
				checkScript: tc.checkScript,
				listResult:  tc.list,
				listErr:     tc.listErr,
			}
			p := &installPipeline{
				queue:   NewOperationQueue(),
				ac:      stub,
				appsDir: appsDir,
			}

			ctx, cancel := context.WithCancel(context.Background())
			if tc.preCancel {
				cancel()
			} else {
				t.Cleanup(cancel)
			}

			err := p.verifyInstalled(ctx, appName)

			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErrSub != "" && err != nil {
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
			}
			if tc.wantCtxErr && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("expected ctx error, got %v", err)
			}
			if got := atomic.LoadInt32(&stub.nCheck); got != tc.wantNCheck {
				t.Errorf("nCheck = %d, want %d", got, tc.wantNCheck)
			}
			if got := atomic.LoadInt32(&stub.nList); got != tc.wantNList {
				t.Errorf("nList = %d, want %d", got, tc.wantNList)
			}
		})
	}
}

// TestResolveVolumeFor locks the update-pinning contract: an update targets the
// app's CURRENT volume and fails closed when it cannot be determined, while a
// fresh install uses the default volume. This is the core guard against the
// cross-volume relocation data loss in conversun/fnos-apps#189.
func TestResolveVolumeFor(t *testing.T) {
	t.Run("update pins to the app's current volume", func(t *testing.T) {
		stub := &stubAppCenter{appVolIdx: 2, appVolFound: true}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolumeFor("update", "emby")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 2 {
			t.Errorf("volume = %d, want 2", got)
		}
	})

	t.Run("update fails closed when current volume is unresolvable", func(t *testing.T) {
		stub := &stubAppCenter{appVolFound: false}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		_, err := p.resolveVolumeFor("update", "emby")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "已中止更新") {
			t.Errorf("error %q missing abort reason", err.Error())
		}
	})

	t.Run("install uses the default volume, not the pin", func(t *testing.T) {
		stub := &stubAppCenter{appVolIdx: 2, appVolFound: true}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolumeFor("install", "emby")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 1 {
			t.Errorf("volume = %d, want 1 (DefaultVolume)", got)
		}
	})
}

// TestPreflightInstall locks the fail-closed guard that runs BEFORE the
// destructive install-local step: a missing or full target volume aborts the
// operation so the app is never left uninstalled with orphaned data.
func TestPreflightInstall(t *testing.T) {
	dir := t.TempDir()
	fpk := filepath.Join(dir, "app.fpk")
	if err := os.WriteFile(fpk, make([]byte, 1000), 0o600); err != nil {
		t.Fatalf("write fpk: %v", err)
	}

	t.Run("passes when target volume is mounted with ample space", func(t *testing.T) {
		stub := &stubAppCenter{volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1", FreeBytes: 1 << 40}}}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.preflightInstall(1, fpk); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("aborts when target volume is not mounted", func(t *testing.T) {
		stub := &stubAppCenter{volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1", FreeBytes: 1 << 40}}}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		err := p.preflightInstall(9, fpk)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "不可用") {
			t.Errorf("error %q missing unavailable reason", err.Error())
		}
	})

	t.Run("aborts when target volume lacks free space", func(t *testing.T) {
		stub := &stubAppCenter{volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1", FreeBytes: 100}}}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		err := p.preflightInstall(1, fpk)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "空间不足") {
			t.Errorf("error %q missing space reason", err.Error())
		}
	})
}

// TestSetDefaultVolume locks that the update volume pin drives the documented
// default-volume lever and surfaces CLI failures so the caller can fail closed
// before the destructive install-local (conversun/fnos-apps#189).
func TestSetDefaultVolume(t *testing.T) {
	t.Run("propagates the target volume to the CLI", func(t *testing.T) {
		stub := &stubAppCenter{}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.setDefaultVolume(3); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(stub.setVolCalls) != 1 || stub.setVolCalls[0] != 3 {
			t.Fatalf("SetDefaultVolume calls = %v, want [3]", stub.setVolCalls)
		}
	})

	t.Run("surfaces a CLI failure", func(t *testing.T) {
		stub := &stubAppCenter{setVolErr: errors.New("cli boom")}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.setDefaultVolume(1); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
