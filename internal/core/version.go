package core

import (
	"strconv"
	"strings"
	"unicode"
)

func CompareVersions(a, b string) int {
	aParts := strings.Split(strings.TrimSpace(a), ".")
	bParts := strings.Split(strings.TrimSpace(b), ".")

	maxParts := len(aParts)
	if len(bParts) > maxParts {
		maxParts = len(bParts)
	}

	for i := range maxParts {
		left := versionPartAsInt(aParts, i)
		right := versionPartAsInt(bParts, i)
		switch {
		case left < right:
			return -1
		case left > right:
			return 1
		}
	}

	return 0
}

// CompareFpkVersions compares two fpk_version strings.
// Returns:
//   - 0 if versions are equal (up-to-date)
//   - -1 if upgrade is needed (a < b)
//   - 1 if downgrade (a > b)
//
// Comparison logic:
// 1. If strings are equal, return 0
// 2. Extract base version (everything before first '-')
// 3. Compare base versions using CompareVersions
// 4. If bases differ, return that result
// 5. If bases are same but strings differ, return -1 (different revision = update available)
func CompareFpkVersions(a, b string) int {
	// If strings are exactly equal, versions match
	if a == b {
		return 0
	}

	// Extract base versions (everything before first '-')
	baseA := a
	if idx := strings.Index(a, "-"); idx >= 0 {
		baseA = a[:idx]
	}

	baseB := b
	if idx := strings.Index(b, "-"); idx >= 0 {
		baseB = b[:idx]
	}

	// Compare base versions
	baseCmp := CompareVersions(baseA, baseB)
	if baseCmp != 0 {
		return baseCmp
	}

	// Base versions are same but full strings differ (different revision)
	// This means an update is available
	return -1
}

func versionPartAsInt(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}

	segment := strings.TrimSpace(parts[index])
	if segment == "" {
		return 0
	}

	numeric := extractLeadingDigits(segment)
	if numeric == "" {
		return 0
	}

	v, err := strconv.Atoi(numeric)
	if err != nil {
		return 0
	}

	return v
}

func extractLeadingDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsDigit(r) {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}
