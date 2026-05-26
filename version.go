package main

import (
	"runtime/debug"
	"strings"
	"time"
)

const unknownVersion = "unknown"

// version can be set by builds with:
//
//	go build -ldflags "-X main.version=YYYYMMDD+<git-sha1>"
var version string

func appVersion() string {
	if strings.TrimSpace(version) != "" {
		return strings.TrimSpace(version)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return unknownVersion
	}
	if v := appVersionFromBuildSettings(info.Settings); v != "" {
		return v
	}
	return unknownVersion
}

func appVersionFromBuildSettings(settings []debug.BuildSetting) string {
	var revision, commitTime string
	for _, setting := range settings {
		switch setting.Key {
		case "vcs.revision":
			revision = strings.TrimSpace(setting.Value)
		case "vcs.time":
			commitTime = strings.TrimSpace(setting.Value)
		}
	}
	if revision == "" || commitTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, commitTime)
	if err != nil {
		return ""
	}
	return t.Format("20060102") + "+" + revision
}
