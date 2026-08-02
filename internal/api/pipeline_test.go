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

	"fnos-store/internal/config"
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

	setVolCalls    []int
	setVolErr      error
	upgradeBlocked bool
	// curVol is the volume the stub reports from DefaultVolume(). A successful
	// SetDefaultVolume updates it; setVolIgnored models the real fnOS defect
	// where the setter exits 0 but the value never changes.
	curVol        int
	setVolIgnored bool
	getVolErr     error

	nCheck           int32
	nList            int32
	nInstallFpk      int32
	nInstallLocal    int32
	nUpgradeFpk      int32
	nInstallWizard   int32
	upgradeErr       error
	installWizardErr error
	wizard           *platform.AppWizard
	lastParams       []platform.WizardParam
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

func (s *stubAppCenter) Status(string) (string, error) { return "", nil }
func (s *stubAppCenter) InstallFpk(string, int) error {
	atomic.AddInt32(&s.nInstallFpk, 1)
	return nil
}

func (s *stubAppCenter) InstallLocal(string, int, bool) error {
	atomic.AddInt32(&s.nInstallLocal, 1)
	return nil
}
func (s *stubAppCenter) Uninstall(string) error { return nil }
func (s *stubAppCenter) Start(string) error     { return nil }
func (s *stubAppCenter) Stop(string) error      { return nil }
func (s *stubAppCenter) DefaultVolume() (int, error) {
	if s.getVolErr != nil {
		return 0, s.getVolErr
	}
	if s.curVol == 0 {
		return 1, nil
	}
	return s.curVol, nil
}
func (s *stubAppCenter) ListVolumes() ([]platform.VolumeInfo, error) { return s.volumes, nil }
func (s *stubAppCenter) FetchWizard(_ context.Context, _ string) (*platform.AppWizard, error) {
	return s.wizard, nil
}

func (s *stubAppCenter) InstallFpkWithWizard(_ context.Context, _ string, _ int, params []platform.WizardParam) error {
	atomic.AddInt32(&s.nInstallWizard, 1)
	s.lastParams = params
	return s.installWizardErr
}

func (s *stubAppCenter) UpgradeFpk(_ context.Context, _ string, _ []platform.WizardParam) error {
	atomic.AddInt32(&s.nUpgradeFpk, 1)
	return s.upgradeErr
}

func (s *stubAppCenter) UpgradeCapability() platform.UpgradeCapability {
	if s.upgradeBlocked {
		return platform.UpgradeCapability{Allowed: false, PlatformVersion: "1.2.0203", Reason: "该 fnOS 版本更新会删除应用数据"}
	}
	return platform.UpgradeCapability{Allowed: true, PlatformVersion: "test"}
}

func (s *stubAppCenter) AppInstallVolume(string) (int, bool, error) {
	return s.appVolIdx, s.appVolFound, s.appVolErr
}
func (s *stubAppCenter) SetDefaultVolume(v int) error {
	s.setVolCalls = append(s.setVolCalls, v)
	if s.setVolErr != nil {
		return s.setVolErr
	}
	if !s.setVolIgnored {
		s.curVol = v
	}
	return nil
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
		stub := &stubAppCenter{
			appVolIdx:   2,
			appVolFound: true,
			volumes:     []platform.VolumeInfo{{Index: 1, Path: "/vol1"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolumeFor("install", "emby")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 1 {
			t.Errorf("volume = %d, want 1 (DefaultVolume)", got)
		}
	})

	// fnOS 1.2.0203 returns a default volume of 0 from the CLI even though its
	// own database holds a valid index, and vol0 does not exist. Installing
	// onto it always fails, so surface the one lever the user can actually
	// pull instead of a generic downstream error.
	t.Run("install rejects a default volume that is not mounted", func(t *testing.T) {
		stub := &stubAppCenter{
			curVol:  9,
			volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1"}, {Index: 2, Path: "/vol2"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		_, err := p.resolveVolumeFor("install", "emby")
		if err == nil {
			t.Fatal("expected error for an unmounted default volume, got nil")
		}
		if !strings.Contains(err.Error(), "设置") {
			t.Errorf("error %q should point the user at settings", err.Error())
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

	// THE regression that motivated the read-back. On fnOS 1.2.0203 the real
	// `appcenter-cli default-volume <n>` exits 0, prints the OLD value, and
	// changes nothing. Trusting the setter's nil error let an update walk into
	// install-local's uninstall step with an unpinned (invalid) volume, so the
	// reinstall failed and the app was destroyed (conversun/fnos-apps#189).
	t.Run("fails closed when the setter is silently ignored", func(t *testing.T) {
		stub := &stubAppCenter{curVol: 0, setVolIgnored: true}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		err := p.setDefaultVolume(2)
		if err == nil {
			t.Fatal("expected error when default-volume set is a no-op, got nil")
		}
		if !strings.Contains(err.Error(), "未生效") {
			t.Errorf("error %q should report the pin did not take effect", err.Error())
		}
	})

	// Single-volume systems get NO exemption here. Relocation is impossible
	// with one volume, but the pin verification is not what protects them —
	// on fnOS 1.2.0203 the update destroys the app regardless (error 10237),
	// so the refusal now lives in requireSafeUpgrade(). Skipping verification
	// here would only have let those users reach the destroyer sooner.
	t.Run("single volume does not exempt the pin verification", func(t *testing.T) {
		stub := &stubAppCenter{
			curVol:        -1, // broken getter, as measured on fnOS 1.2.0203
			setVolIgnored: true,
			volumes:       []platform.VolumeInfo{{Index: 1, Path: "/vol1"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.setDefaultVolume(1); err == nil {
			t.Fatal("expected the pin verification to still fail closed")
		}
	})

	t.Run("fails closed when the value cannot be read back", func(t *testing.T) {
		stub := &stubAppCenter{getVolErr: errors.New("read boom")}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.setDefaultVolume(2); err == nil {
			t.Fatal("expected error when read-back fails, got nil")
		}
	})
}

// TestResolveVolume locks the fresh-install volume choice on a system whose
// fnOS default-volume getter is broken. Measured on fnOS 1.2.0203: the getter
// returns 0 while the daemon's own DB holds 1, and vol0 does not exist.
//
// Dead-ending the user there is bad UX — the Settings default reads "系统默认",
// which looks correct and gives no hint it is the thing failing. valfar7 hit
// exactly that on 1.7.14 (conversun/fnos-apps#189). When only one volume is
// mounted there is no ambiguity, so fall back to it instead of demanding a
// choice the user cannot know they must make.
func TestResolveVolume(t *testing.T) {
	t.Run("uses the configured volume when set", func(t *testing.T) {
		stub := &stubAppCenter{curVol: 9, volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1"}}}
		p := &installPipeline{
			queue:     NewOperationQueue(),
			ac:        stub,
			configMgr: config.NewManager(t.TempDir()),
		}
		cfg, err := p.configMgr.LoadConfig()
		if err != nil {
			t.Fatal(err)
		}
		cfg.InstallVolume = 2
		if err := p.configMgr.SaveConfig(cfg); err != nil {
			t.Fatal(err)
		}
		got, err := p.resolveVolume()
		if err != nil || got != 2 {
			t.Fatalf("resolveVolume() = (%d, %v), want (2, nil)", got, err)
		}
	})

	t.Run("accepts a usable fnOS default", func(t *testing.T) {
		stub := &stubAppCenter{
			curVol:  2,
			volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1"}, {Index: 2, Path: "/vol2"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolume()
		if err != nil || got != 2 {
			t.Fatalf("resolveVolume() = (%d, %v), want (2, nil)", got, err)
		}
	})

	// THE regression: getter says vol0, only /vol1 exists.
	t.Run("falls back to the only mounted volume when the default is unusable", func(t *testing.T) {
		stub := &stubAppCenter{curVol: -1, volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1"}}}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolume()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != 1 {
			t.Errorf("volume = %d, want 1 (the only mounted volume)", got)
		}
	})

	// With several volumes there is no safe guess — ask, and say where to look.
	t.Run("asks the user when several volumes exist and the default is unusable", func(t *testing.T) {
		stub := &stubAppCenter{
			curVol:  -1,
			volumes: []platform.VolumeInfo{{Index: 1, Path: "/vol1"}, {Index: 2, Path: "/vol2"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		_, err := p.resolveVolume()
		if err == nil {
			t.Fatal("expected an error when the choice is ambiguous, got nil")
		}
		if !strings.Contains(err.Error(), "应用安装位置") {
			t.Errorf("error %q should name the settings field to change", err.Error())
		}
	})

	t.Run("falls back when the getter itself fails", func(t *testing.T) {
		stub := &stubAppCenter{
			getVolErr: errors.New("cli boom"),
			volumes:   []platform.VolumeInfo{{Index: 1, Path: "/vol1"}},
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		got, err := p.resolveVolume()
		if err != nil || got != 1 {
			t.Fatalf("resolveVolume() = (%d, %v), want (1, nil)", got, err)
		}
	})
}

// TestVerifyPayloadLanded locks the filesystem proof that an install/update
// actually produced files at the shipped version. The appcenter control plane
// cannot be trusted for this: `check` returned "Installed" for an app whose
// directory had been deleted, and an update reported 操作完成 while the on-disk
// manifest still held the OLD version (conversun/fnos-apps#189).
func TestVerifyPayloadLanded(t *testing.T) {
	const appName = "filebrowser"

	// newApp lays out the /var/apps/<app> shape the checker reads: a manifest
	// plus a target symlink into the "volume" directory holding the payload.
	// newAppRev lays out an app whose manifest carries BOTH version and
	// fpk_version, the shape a repackaged (-rN) build actually ships.
	newAppRev := func(t *testing.T, version, fpkVersion string) string {
		t.Helper()
		root := t.TempDir()
		appsDir := filepath.Join(root, "apps")
		volDir := filepath.Join(root, "vol1", "@appcenter", appName)
		if err := os.MkdirAll(filepath.Join(appsDir, appName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(volDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(volDir, appName), []byte("binary"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(volDir, filepath.Join(appsDir, appName, "target")); err != nil {
			t.Fatal(err)
		}
		manifest := "appname         = " + appName + "\nversion         = " + version + "\n"
		if fpkVersion != "" {
			manifest += "fpk_version     = " + fpkVersion + "\n"
		}
		if err := os.WriteFile(filepath.Join(appsDir, appName, "manifest"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return appsDir
	}

	newApp := func(t *testing.T, version string, payload bool) (appsDir string, volDir string) {
		t.Helper()
		root := t.TempDir()
		appsDir = filepath.Join(root, "apps")
		volDir = filepath.Join(root, "vol1", "@appcenter", appName)
		if err := os.MkdirAll(filepath.Join(appsDir, appName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(volDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if payload {
			if err := os.WriteFile(filepath.Join(volDir, appName), []byte("binary"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Symlink(volDir, filepath.Join(appsDir, appName, "target")); err != nil {
			t.Fatal(err)
		}
		manifest := "appname         = " + appName + "\nversion         = " + version + "\n"
		if err := os.WriteFile(filepath.Join(appsDir, appName, "manifest"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		return appsDir, volDir
	}

	t.Run("accepts a real install at the shipped version", func(t *testing.T) {
		appsDir, _ := newApp(t, "2.63.19", true)
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		if err := p.verifyPayloadLanded(appName, 0, "2.63.19"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// The exact false-success observed on the VM: install-local exited 0, the
	// control plane said Installed, but nothing was replaced.
	t.Run("rejects an update whose version never changed", func(t *testing.T) {
		appsDir, _ := newApp(t, "2.63.18", true)
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		err := p.verifyPayloadLanded(appName, 0, "2.63.19")
		if err == nil {
			t.Fatal("expected error when the version did not change, got nil")
		}
		if !strings.Contains(err.Error(), "2.63.19") {
			t.Errorf("error %q should name the expected version", err.Error())
		}
	})

	// The store itself was found in this state: process alive, directory gone.
	t.Run("rejects an empty install directory", func(t *testing.T) {
		appsDir, _ := newApp(t, "2.63.19", false)
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		err := p.verifyPayloadLanded(appName, 0, "2.63.19")
		if err == nil {
			t.Fatal("expected error for an empty install dir, got nil")
		}
		if !strings.Contains(err.Error(), "为空") {
			t.Errorf("error %q should report the empty payload", err.Error())
		}
	})

	t.Run("rejects a missing install directory", func(t *testing.T) {
		appsDir, volDir := newApp(t, "2.63.19", true)
		if err := os.RemoveAll(volDir); err != nil {
			t.Fatal(err)
		}
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		if err := p.verifyPayloadLanded(appName, 0, "2.63.19"); err == nil {
			t.Fatal("expected error for a missing install dir, got nil")
		}
	})

	// A repackaged build ships fpk_version=1.9.3-r2 while the manifest's own
	// version stays 1.9.3. runStandard passes app.FpkVersion as wantVersion, so
	// comparing it against manifest `version` would reject a PERFECTLY GOOD
	// install. 29 of the 145 catalogued apps carry a -rN suffix (1panel, alist,
	// gitea, gopeed, embyserver ...), so this is a fifth of the catalog.
	t.Run("accepts a revision package by fpk_version", func(t *testing.T) {
		appsDir := newAppRev(t, "1.9.3", "1.9.3-r2")
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		if err := p.verifyPayloadLanded(appName, 0, "1.9.3-r2"); err != nil {
			t.Fatalf("revision package must be accepted, got: %v", err)
		}
	})

	t.Run("still rejects a revision package that did not upgrade", func(t *testing.T) {
		appsDir := newAppRev(t, "1.9.3", "1.9.3-r1")
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		err := p.verifyPayloadLanded(appName, 0, "1.9.3-r2")
		if err == nil {
			t.Fatal("expected error when fpk_version did not change, got nil")
		}
		if !strings.Contains(err.Error(), "1.9.3-r2") {
			t.Errorf("error %q should name the expected version", err.Error())
		}
	})

	t.Run("falls back to version when no fpk_version is shipped", func(t *testing.T) {
		appsDir := newAppRev(t, "2.63.19", "")
		p := &installPipeline{queue: NewOperationQueue(), ac: &stubAppCenter{}, appsDir: appsDir}
		if err := p.verifyPayloadLanded(appName, 0, "2.63.19"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Relocation is the data-orphaning half of #189: the app lands on a volume
	// other than the one we pinned, leaving its data behind on the old one.
	t.Run("rejects a payload that landed on the wrong volume", func(t *testing.T) {
		appsDir, volDir := newApp(t, "2.63.19", true)
		// Resolve first: on macOS t.TempDir() sits under a /var -> /private/var
		// symlink, and verifyPayloadLanded compares the RESOLVED payload path.
		resolvedVol, err := filepath.EvalSymlinks(volDir)
		if err != nil {
			t.Fatal(err)
		}
		volRoot := filepath.Dir(filepath.Dir(resolvedVol)) // .../vol1
		stub := &stubAppCenter{volumes: []platform.VolumeInfo{{Index: 2, Path: volRoot}}}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub, appsDir: appsDir}
		err = p.verifyPayloadLanded(appName, 1, "2.63.19")
		if err == nil {
			t.Fatal("expected error when the app landed on another volume, got nil")
		}
		if !strings.Contains(err.Error(), "vol2") {
			t.Errorf("error %q should name the actual volume", err.Error())
		}
	})
}

// TestUpgradeGuardBlocksDestructivePath locks the refusal added after a user
// lost apps on fnOS 1.2.0203 (conversun/fnos-apps#189).
//
// fnOS has no data-preserving upgrade command: `install-fpk` refuses an
// already-installed app outright, and `install-local` implements an upgrade as
// uninstall-then-reinstall whose reinstall fails with error 10237 on that
// build. Reproduced twice on a live box — gopeed went from running to gone,
// program directory AND @appdata deleted.
//
// The guard therefore has to fire BEFORE anything is downloaded or installed,
// because nothing downstream can undo the uninstall.
func TestUpgradeGuardBlocksDestructivePath(t *testing.T) {
	newPipeline := func(blocked bool) (*installPipeline, *stubAppCenter) {
		stub := &stubAppCenter{
			upgradeBlocked: blocked,
			appVolIdx:      1,
			appVolFound:    true,
			volumes:        []platform.VolumeInfo{{Index: 1, Path: "/vol1"}},
			checkScript:    []stubCheckResult{{installed: true}},
		}
		return &installPipeline{queue: NewOperationQueue(), ac: stub}, stub
	}

	t.Run("update is refused before any destructive call", func(t *testing.T) {
		p, stub := newPipeline(true)
		if err := p.requireSafeUpgrade(); err == nil {
			t.Fatal("expected the update to be refused on an unsafe build")
		}
		if n := atomic.LoadInt32(&stub.nInstallLocal); n != 0 {
			t.Errorf("InstallLocal called %d times; must never run when blocked", n)
		}
		if n := atomic.LoadInt32(&stub.nInstallFpk); n != 0 {
			t.Errorf("InstallFpk called %d times; must never run when blocked", n)
		}
	})

	t.Run("refusal explains the manual route", func(t *testing.T) {
		p, _ := newPipeline(true)
		err := p.requireSafeUpgrade()
		if err == nil {
			t.Fatal("expected an error")
		}
		if !strings.Contains(err.Error(), "删除") {
			t.Errorf("error %q should explain the data-loss risk", err.Error())
		}
	})

	// Fresh installs stay available: there is no existing app to destroy, and
	// install-fpk on a not-installed app works fine on the affected build.
	t.Run("safe builds are unaffected", func(t *testing.T) {
		p, _ := newPipeline(false)
		if err := p.requireSafeUpgrade(); err != nil {
			t.Fatalf("update must proceed on a safe build, got: %v", err)
		}
	})
}

// TestUpdateUsesDaemonUpgradeNotInstallLocal locks the routing that keeps an
// update from destroying the app.
//
// fnOS has two upgrade mechanisms with opposite outcomes: the daemon's RPC
// upgrade preserves @appdata and can roll back, while install-local is
// uninstall-then-reinstall and on fnOS 1.2.0203 always loses the app. So an
// update must reach UpgradeFpk, and a FAILED update must NOT retry through
// InstallFpk — falling back would turn a clean failure into data loss
// (conversun/fnos-apps#189).
func TestUpdateUsesDaemonUpgradeNotInstallLocal(t *testing.T) {
	t.Run("update routes to UpgradeFpk", func(t *testing.T) {
		stub := &stubAppCenter{}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.upgradeFpk(context.Background(), "/tmp/x.fpk"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n := atomic.LoadInt32(&stub.nUpgradeFpk); n != 1 {
			t.Errorf("UpgradeFpk called %d times, want 1", n)
		}
		if n := atomic.LoadInt32(&stub.nInstallFpk); n != 0 {
			t.Errorf("InstallFpk called %d times; the destructive path must not run for updates", n)
		}
	})

	t.Run("a failed upgrade does not fall back to install-local", func(t *testing.T) {
		stub := &stubAppCenter{upgradeErr: errSimulatedUpgrade}
		p := &installPipeline{queue: NewOperationQueue(), ac: stub}
		if err := p.upgradeFpk(context.Background(), "/tmp/x.fpk"); err == nil {
			t.Fatal("expected the upgrade error to surface")
		}
		if n := atomic.LoadInt32(&stub.nInstallFpk); n != 0 {
			t.Errorf("InstallFpk called %d times after a failed upgrade; that would destroy the app", n)
		}
		if n := atomic.LoadInt32(&stub.nInstallLocal); n != 0 {
			t.Errorf("InstallLocal called %d times after a failed upgrade", n)
		}
	})
}

// TestChooseInstallRoute locks the channel every operation installs/upgrades
// through: updates always take the daemon's data-preserving upgrade, fresh
// installs take the daemon's install channel when reachable (avoiding
// install-local's code-10237 chown failures, #227/#228), and only a box whose
// daemon is unreachable falls back to install-local for a fresh install.
func TestChooseInstallRoute(t *testing.T) {
	cases := []struct {
		name      string
		opName    string
		daemonUp  bool
		want      installRoute
	}{
		{"update uses the daemon upgrade even when the daemon is down", "update", false, routeDaemonUpgrade},
		{"update uses the daemon upgrade when the daemon is up", "update", true, routeDaemonUpgrade},
		{"fresh install uses the daemon install when the daemon is up", "install", true, routeDaemonInstall},
		{"fresh install falls back to install-local only when the daemon is down", "install", false, routeInstallLocal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chooseInstallRoute(c.opName, c.daemonUp); got != c.want {
				t.Errorf("chooseInstallRoute(%q, %v) = %v, want %v", c.opName, c.daemonUp, got, c.want)
			}
		})
	}
}

var errSimulatedUpgrade = errors.New("simulated upgrade failure")
