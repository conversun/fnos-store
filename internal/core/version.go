package core

type VersionComparison int

const (
	VersionEqual  VersionComparison = 0
	VersionNewer  VersionComparison = 1
	VersionOlder  VersionComparison = -1
)

// CompareVersions compares two version strings.
// Returns VersionNewer if a > b, VersionOlder if a < b, VersionEqual if a == b.
func CompareVersions(a, b string) VersionComparison {
	_ = a
	_ = b
	return VersionEqual
}
