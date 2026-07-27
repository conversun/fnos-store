//go:build !linux

package platform

// UpgradeCapability always permits updates in the macOS dev mock.
func (m *MockAppCenter) UpgradeCapability() UpgradeCapability {
	return UpgradeCapability{Allowed: true, PlatformVersion: "mock"}
}
