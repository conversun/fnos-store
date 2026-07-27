package platform

import "context"

// InstalledApp represents an app installed via appcenter-cli.
type InstalledApp struct {
	AppName     string
	Version     string
	DisplayName string
	Status      string // "running" or "stopped"
}

// VolumeInfo represents an available installation volume.
type VolumeInfo struct {
	Index      int
	Path       string
	TotalBytes uint64
	FreeBytes  uint64
}

// AppCenter abstracts appcenter-cli operations.
// The real implementation calls appcenter-cli on Linux;
// the mock implementation simulates it for development on macOS.
type AppCenter interface {
	// List returns all installed applications.
	List() ([]InstalledApp, error)

	// Check returns true if the given app is installed.
	Check(appname string) (bool, error)

	// Status returns the running status of an app ("running" or "stopped").
	Status(appname string) (string, error)

	// InstallFpk installs or upgrades an app from an fpk file on the given volume.
	// It extracts the fpk and uses install-local internally for upgrade support.
	InstallFpk(fpkPath string, volume int) error

	// InstallLocal installs or upgrades an app from an extracted fpk directory.
	// When detach is true, the process is launched in a new session so it
	// survives the caller being killed (used for self-update).
	InstallLocal(dir string, volume int, detach bool) error

	// Uninstall removes an installed app.
	Uninstall(appname string) error

	// Start starts an installed app.
	Start(appname string) error

	// Stop stops a running app.
	Stop(appname string) error

	// DefaultVolume returns the default installation volume index.
	DefaultVolume() (int, error)

	// SetDefaultVolume sets fnOS's default installation volume. This is the
	// documented lever for install placement; install-local honors it even on
	// builds where the undocumented -v flag is ignored.
	SetDefaultVolume(volume int) error

	// ListVolumes returns all available installation volumes.
	ListVolumes() ([]VolumeInfo, error)

	// AppInstallVolume resolves the volume index an app is CURRENTLY installed
	// on, derived from its on-disk layout independently of appcenter-cli output.
	// found is false when the app is not installed or its volume cannot be
	// determined. Updates MUST pin to this volume so an app is never relocated
	// off its existing data.
	AppInstallVolume(appname string) (int, bool, error)

	// UpgradeCapability reports whether this fnOS build can update an
	// installed app without destroying it. Some builds cannot — see
	// upgrade.go.
	UpgradeCapability() UpgradeCapability

	// UpgradeFpk upgrades an ALREADY-INSTALLED app in place, preserving its
	// data. This is deliberately separate from InstallFpk: InstallFpk goes
	// through install-local, which implements an upgrade as
	// uninstall-then-reinstall and destroys the app when the reinstall fails.
	UpgradeFpk(ctx context.Context, fpkPath string, params []WizardParam) error

	// FetchWizard returns an app's install-time form definition without
	// installing anything, so the UI can ask the same questions the native
	// App Center does.
	FetchWizard(ctx context.Context, fpkPath string) (*AppWizard, error)

	// InstallFpkWithWizard installs a not-yet-installed app, passing the
	// user's answers to the app's own install wizard.
	InstallFpkWithWizard(ctx context.Context, fpkPath string, volume int, params []WizardParam) error
}
