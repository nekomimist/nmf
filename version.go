package main

import (
	_ "embed"
	"strings"
)

// VERSION is shared by source builds and release packaging.
//
//go:embed VERSION
var version string

func appVersion() string {
	return strings.TrimSpace(version)
}
