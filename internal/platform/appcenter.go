package platform

// InstalledApp represents an app installed via appcenter-cli.
type InstalledApp struct {
	AppName     string
	Version     string
	DisplayName string
	Status      string // "running" or "stopped"
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
	InstallFpk(fpkPath string, volume int) error

	// Uninstall removes an installed app.
	Uninstall(appname string) error

	// Start starts an installed app.
	Start(appname string) error

	// Stop stops a running app.
	Stop(appname string) error

	// DefaultVolume returns the default installation volume index.
	DefaultVolume() (int, error)
}
