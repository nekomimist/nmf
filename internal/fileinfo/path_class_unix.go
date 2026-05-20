//go:build !windows
// +build !windows

package fileinfo

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type mountInfoEntry struct {
	mountPoint string
	fsType     string
	majorMinor string
}

func classifyLocalPath(p string) (PathClass, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		if resolved, err := filepath.Abs(abs); err == nil {
			abs = resolved
		}
	}

	entry, ok := bestMountInfoEntry(abs, readProcSelfMountInfo)
	if !ok {
		return PathClass{}, nil
	}
	return PathClass{
		Network:   isNetworkFilesystemType(entry.fsType),
		Removable: isRemovableBlockDevice(entry.majorMinor),
	}, nil
}

func bestMountInfoEntry(p string, read func() ([]mountInfoEntry, error)) (mountInfoEntry, bool) {
	entries, err := read()
	if err != nil {
		return mountInfoEntry{}, false
	}

	cleanPath := filepath.Clean(p)
	var best mountInfoEntry
	bestLen := -1
	for _, entry := range entries {
		mountPoint := filepath.Clean(entry.mountPoint)
		if !pathWithinMount(cleanPath, mountPoint) {
			continue
		}
		if len(mountPoint) > bestLen {
			best = entry
			bestLen = len(mountPoint)
		}
	}
	return best, bestLen >= 0
}

func pathWithinMount(p, mountPoint string) bool {
	if mountPoint == "/" {
		return strings.HasPrefix(p, "/")
	}
	return p == mountPoint || strings.HasPrefix(p, mountPoint+"/")
}

func readProcSelfMountInfo() ([]mountInfoEntry, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []mountInfoEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if entry, ok := parseMountInfoLine(scanner.Text()); ok {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func parseMountInfoLine(line string) (mountInfoEntry, bool) {
	parts := strings.Split(line, " ")
	separator := -1
	for i, part := range parts {
		if part == "-" {
			separator = i
			break
		}
	}
	if separator < 6 || separator+1 >= len(parts) {
		return mountInfoEntry{}, false
	}
	return mountInfoEntry{
		mountPoint: unescapeMountInfoField(parts[4]),
		majorMinor: parts[2],
		fsType:     parts[separator+1],
	}, true
}

func unescapeMountInfoField(field string) string {
	var b strings.Builder
	for i := 0; i < len(field); i++ {
		if field[i] == '\\' && i+3 < len(field) {
			if value, err := strconv.ParseInt(field[i+1:i+4], 8, 32); err == nil {
				b.WriteByte(byte(value))
				i += 3
				continue
			}
		}
		b.WriteByte(field[i])
	}
	return b.String()
}

func isRemovableBlockDevice(majorMinor string) bool {
	parts := strings.SplitN(majorMinor, ":", 2)
	if len(parts) != 2 {
		return false
	}
	data, err := os.ReadFile(filepath.Join("/sys/dev/block", majorMinor, "removable"))
	if err == nil {
		return strings.TrimSpace(string(data)) == "1"
	}
	for {
		devicePath := filepath.Join("/sys/dev/block", majorMinor)
		target, err := os.Readlink(devicePath)
		if err != nil {
			return false
		}
		parent := filepath.Dir(target)
		if parent == "." || parent == "/" {
			return false
		}
		parentBase := filepath.Base(parent)
		if !strings.Contains(parentBase, ":") {
			data, err := os.ReadFile(filepath.Join("/sys/block", parentBase, "removable"))
			return err == nil && strings.TrimSpace(string(data)) == "1"
		}
		majorMinor = parentBase
	}
}
