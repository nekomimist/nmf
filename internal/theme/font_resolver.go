package theme

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/fontscan"

	"nmf/internal/config"
)

type fontResolverLogger struct{}

func (fontResolverLogger) Printf(string, ...interface{}) {}

func resolveThemeFont(themeConfig config.ThemeConfig, debugPrint func(format string, args ...interface{})) fyne.Resource {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}

	if path := strings.TrimSpace(themeConfig.FontPath); path != "" {
		res, err := loadFontResourceFromPath(path)
		if err == nil {
			debugPrint("Theme: Loaded custom font path=%s", path)
			return res
		}
		debugPrint("Theme: FontPath unavailable path=%s err=%v", path, err)
	}

	for _, name := range configuredFontNames(themeConfig.FontName) {
		res, source, err := loadFontResourceByName(name)
		if err == nil {
			debugPrint("Theme: Loaded font name=%s source=%s", name, source)
			return res
		}
		debugPrint("Theme: FontName unavailable name=%s err=%v", name, err)
	}

	return nil
}

func configuredFontNames(configured string) []string {
	name := strings.TrimSpace(configured)
	if name != "" && !strings.EqualFold(name, "auto") {
		names := []string{name}
		for _, fallback := range defaultFontNames(runtime.GOOS) {
			if strings.EqualFold(name, fallback) {
				continue
			}
			names = append(names, fallback)
		}
		return names
	}
	return defaultFontNames(runtime.GOOS)
}

func defaultFontNames(goos string) []string {
	switch goos {
	case "windows":
		return []string{
			"Yu Gothic UI",
			"Meiryo UI",
			"Microsoft YaHei UI",
			"Microsoft JhengHei UI",
			"Malgun Gothic",
			"Segoe UI",
		}
	case "linux":
		return []string{
			"Noto Sans CJK JP",
			"Noto Sans CJK SC",
			"Noto Sans CJK TC",
			"Noto Sans CJK KR",
			"Noto Sans",
			"DejaVu Sans",
		}
	default:
		return []string{
			"Noto Sans",
			"DejaVu Sans",
		}
	}
}

func loadFontResourceByName(name string) (fyne.Resource, string, error) {
	fm := fontscan.NewFontMap(fontResolverLogger{})
	if err := fm.UseSystemFonts(""); err != nil {
		return nil, "", fmt.Errorf("scan system fonts: %w", err)
	}

	locations := fm.FindSystemFonts(name)
	if len(locations) == 0 {
		locations = scanFontLocationsByName(name)
	}
	if len(locations) == 0 {
		return nil, "", fmt.Errorf("font family not found")
	}

	var lastErr error
	for _, location := range locations {
		res, source, err := loadFontResourceFromLocation(name, location)
		if err == nil {
			return res, source, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, "", lastErr
	}
	return nil, "", fmt.Errorf("no usable font files found")
}

func scanFontLocationsByName(name string) []fontscan.Location {
	dirs, err := fontscan.DefaultFontDirectories(fontResolverLogger{})
	if err != nil {
		return nil
	}

	target := font.NormalizeFamily(name)
	seenDirs := make(map[string]bool)
	seenFiles := make(map[string]bool)
	var locations []fontscan.Location
	for _, dir := range dirs {
		for _, scanDir := range fontScanDirectoryCandidates(dir) {
			if seenDirs[scanDir] {
				continue
			}
			seenDirs[scanDir] = true
			locations = append(locations, scanDirectoryForFontName(scanDir, target, seenFiles)...)
		}
	}
	return locations
}

func fontScanDirectoryCandidates(dir string) []string {
	out := []string{dir}
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil && resolved != "" && resolved != dir {
		out = append(out, resolved)
	}
	return out
}

func scanDirectoryForFontName(dir, targetFamily string, seenFiles map[string]bool) []fontscan.Location {
	var locations []fontscan.Location
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		if !isFontFile(path) || seenFiles[path] {
			return nil
		}
		seenFiles[path] = true

		matches := fontFileFamilyMatches(path, targetFamily)
		for _, index := range matches {
			locations = append(locations, fontscan.Location{File: path, Index: index})
		}
		return nil
	})
	return locations
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func fontFileFamilyMatches(path, targetFamily string) []uint16 {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	loaders, err := ot.NewLoaders(file)
	if err != nil {
		return nil
	}

	var matches []uint16
	var buffer []byte
	for index, loader := range loaders {
		desc, nextBuffer := font.Describe(loader, buffer)
		buffer = nextBuffer
		if font.NormalizeFamily(desc.Family) == targetFamily {
			matches = append(matches, uint16(index))
		}
	}
	return matches
}

func loadFontResourceFromLocation(name string, location fontscan.Location) (fyne.Resource, string, error) {
	path := location.File
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttc", ".otc":
		extracted, err := extractCollectionFont(path, location.Index, name)
		if err != nil {
			return nil, "", err
		}
		res, err := loadFontResourceFromPath(extracted)
		if err != nil {
			return nil, "", err
		}
		return res, fmt.Sprintf("%s#%d", path, location.Index), nil
	default:
		if location.Index != 0 {
			return nil, "", fmt.Errorf("non-collection font has index %d", location.Index)
		}
		res, err := loadFontResourceFromPath(path)
		if err != nil {
			return nil, "", err
		}
		return res, path, nil
	}
}

func loadFontResourceFromPath(path string) (fyne.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateFontData(data); err != nil {
		return nil, err
	}
	return fyne.NewStaticResource(filepath.Base(path), data), nil
}

func validateFontData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty font file")
	}
	if _, err := font.ParseTTF(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("parse font: %w", err)
	}
	return nil
}

func extractCollectionFont(path string, index uint16, family string) (string, error) {
	cachePath, err := collectionCachePath(path, index, family)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		if _, err := loadFontResourceFromPath(cachePath); err == nil {
			return cachePath, nil
		}
	}

	data, err := extractCollectionFontData(path, index)
	if err != nil {
		return "", err
	}
	if err := validateFontData(data); err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return "", err
	}
	tmp := cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, cachePath); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return cachePath, nil
}

func extractCollectionFontData(path string, index uint16) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	loaders, err := ot.NewLoaders(file)
	if err != nil {
		return nil, err
	}
	if int(index) >= len(loaders) {
		return nil, fmt.Errorf("font index %d out of range", index)
	}

	loader := loaders[index]
	tags := loader.Tables()
	tables := make([]ot.Table, len(tags))
	for i, tag := range tags {
		content, err := loader.RawTable(tag)
		if err != nil {
			return nil, err
		}
		tables[i] = ot.Table{
			Tag:     tag,
			Content: content,
		}
	}
	return ot.WriteTTF(tables), nil
}

func collectionCachePath(path string, index uint16, family string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s\x00%d\x00%d", path, index, family, info.Size(), info.ModTime().UnixNano())))
	name := hex.EncodeToString(sum[:16]) + ".ttf"
	return filepath.Join(cacheDir, "nekomimist", "nmf", "fonts", name), nil
}
