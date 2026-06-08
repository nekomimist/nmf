package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nmf/internal/config"
)

const debugLogPattern = "nmf-*.log"

func setupRotatingDebugLogging(configPath string, cfg config.DebugConfig) (*os.File, string, error) {
	if !cfg.Enabled {
		return nil, "", nil
	}
	dir := resolveDebugLogDirectory(configPath, cfg.LogDirectory)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, "", fmt.Errorf("creating debug log directory: %w", err)
	}
	file, path, err := openDebugSessionLog(dir, time.Now(), os.Getpid())
	if err != nil {
		return nil, "", err
	}
	debugMode = true
	log.SetOutput(io.MultiWriter(file, os.Stderr))
	debugPrint("App: version=%s", appVersion())
	debugPrint("Logger: debug log started path=%s time=%s", path, time.Now().Format(time.RFC3339))
	if err := pruneDebugLogs(dir, cfg.MaxLogFiles); err != nil {
		debugPrint("Logger: prune failed dir=%s err=%v", dir, err)
	}
	return file, path, nil
}

func resolveDebugLogDirectory(configPath, configured string) string {
	if strings.TrimSpace(configured) == "" {
		return filepath.Join(filepath.Dir(configPath), "logs")
	}
	if filepath.IsAbs(configured) {
		return configured
	}
	return filepath.Join(filepath.Dir(configPath), configured)
}

func debugLogFileName(now time.Time, pid int) string {
	return fmt.Sprintf("nmf-%s-%d.log", now.Format("20060102-150405"), pid)
}

func openDebugSessionLog(dir string, now time.Time, pid int) (*os.File, string, error) {
	base := strings.TrimSuffix(debugLogFileName(now, pid), ".log")
	for i := 0; ; i++ {
		name := base + ".log"
		if i > 0 {
			name = fmt.Sprintf("%s-%d.log", base, i)
		}
		path := filepath.Join(dir, name)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err == nil {
			return file, path, nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
}

func pruneDebugLogs(dir string, maxFiles int) error {
	if maxFiles <= 0 {
		return nil
	}
	matches, err := filepath.Glob(filepath.Join(dir, debugLogPattern))
	if err != nil {
		return err
	}
	if len(matches) <= maxFiles {
		return nil
	}
	sort.Slice(matches, func(i, j int) bool {
		left, leftErr := os.Stat(matches[i])
		right, rightErr := os.Stat(matches[j])
		if leftErr != nil || rightErr != nil {
			return matches[i] < matches[j]
		}
		if left.ModTime().Equal(right.ModTime()) {
			return matches[i] < matches[j]
		}
		return left.ModTime().Before(right.ModTime())
	})
	for _, path := range matches[:len(matches)-maxFiles] {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
