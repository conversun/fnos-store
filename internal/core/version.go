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
