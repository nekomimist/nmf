package theme

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/fontscan"

	"nmf/internal/config"
)

type fontResolverLogger struct{}

func (fontResolverLogger) Printf(string, ...interface{}) {}

type fontCatalog map[string][]fontscan.Location

type describedFont struct {
	index       uint16
	description font.Description
}

// fontResolver shares the expensive system-font discovery between the UI and
// monospace font lookups performed while constructing one CustomTheme.
type fontResolver struct {
	systemOnce        sync.Once
	systemMu          sync.Mutex
	systemMap         *fontscan.FontMap
	systemErr         error
	loadSystemFontMap func() (*fontscan.FontMap, error)

	catalogOnce sync.Once
	catalog     fontCatalog
	catalogErr  error
	scanCatalog func() (fontCatalog, error)
}

func newFontResolver() *fontResolver {
	return &fontResolver{
		loadSystemFontMap: loadSystemFontMap,
		scanCatalog:       scanSystemFontCatalog,
	}
}

func loadSystemFontMap() (*fontscan.FontMap, error) {
	fm := fontscan.NewFontMap(fontResolverLogger{})
	if err := fm.UseSystemFonts(""); err != nil {
		return nil, err
	}
	return fm, nil
}

func (r *fontResolver) resolveThemeFont(themeConfig config.ThemeConfig, debugPrint func(format string, args ...interface{})) fyne.Resource {
	return r.resolveConfiguredFont(themeConfig.FontPath, themeConfig.FontName,
		defaultFontNames(runtime.GOOS), "Font", debugPrint)
}

func (r *fontResolver) resolveThemeMonospaceFont(themeConfig config.ThemeConfig, debugPrint func(format string, args ...interface{})) fyne.Resource {
	return r.resolveConfiguredFont(themeConfig.MonospaceFontPath, themeConfig.MonospaceFontName,
		defaultMonospaceFontNames(runtime.GOOS), "MonospaceFont", debugPrint)
}

func (r *fontResolver) resolveConfiguredFont(pathConfig, nameConfig string, defaults []string, logPrefix string, debugPrint func(format string, args ...interface{})) fyne.Resource {
	if debugPrint == nil {
		debugPrint = func(string, ...interface{}) {}
	}

	if path := strings.TrimSpace(pathConfig); path != "" {
		res, err := loadFontResourceFromPath(path)
		if err == nil {
			debugPrint("Theme: Loaded custom %s path=%s", logPrefix, path)
			return res
		}
		debugPrint("Theme: %sPath unavailable path=%s err=%v", logPrefix, path, err)
	}

	for _, name := range configuredFontNames(nameConfig, defaults) {
		res, source, err := r.loadFontResourceByName(name)
		if err == nil {
			debugPrint("Theme: Loaded %s name=%s source=%s", logPrefix, name, source)
			return res
		}
		debugPrint("Theme: %sName unavailable name=%s err=%v", logPrefix, name, err)
	}

	return nil
}

func configuredFontNames(configured string, defaults []string) []string {
	name := strings.TrimSpace(configured)
	if name != "" && !strings.EqualFold(name, "auto") {
		names := []string{name}
		for _, fallback := range defaults {
			if strings.EqualFold(name, fallback) {
				continue
			}
			names = append(names, fallback)
		}
		return names
	}
	return defaults
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

// defaultMonospaceFontNames lists the built-in monospace fallback names in
// preference order. CJK-capable fixed-pitch faces come first so file/path
// text with Japanese, Chinese, or Korean glyphs still renders monospaced;
// ASCII-only faces are listed last as a final fallback.
func defaultMonospaceFontNames(goos string) []string {
	switch goos {
	case "windows":
		return []string{"BIZ UDGothic", "MS Gothic", "Cascadia Mono", "Consolas"}
	case "linux":
		return []string{
			"Noto Sans Mono CJK JP", "Noto Sans Mono CJK SC",
			"Noto Sans Mono CJK TC", "Noto Sans Mono CJK KR",
			"Noto Sans Mono", "Noto Mono",
			"DejaVu Sans Mono", "Liberation Mono", "Ubuntu Mono",
		}
	default:
		return []string{"Noto Sans Mono", "DejaVu Sans Mono", "Liberation Mono"}
	}
}

func (r *fontResolver) loadFontResourceByName(name string) (fyne.Resource, string, error) {
	locations, discoveryErr := r.fontLocations(name)
	if len(locations) == 0 {
		if discoveryErr != nil {
			return nil, "", fmt.Errorf("scan system fonts: %w", discoveryErr)
		}
		return nil, "", fmt.Errorf("font family not found")
	}

	sortFontLocationsByRegularPreference(locations, describeFontLocation)

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

func (r *fontResolver) fontLocations(name string) ([]fontscan.Location, error) {
	if r == nil {
		return nil, fmt.Errorf("font resolver is nil")
	}
	if r.loadSystemFontMap != nil {
		r.systemOnce.Do(func() {
			r.systemMap, r.systemErr = r.loadSystemFontMap()
		})
	}
	if r.systemMap != nil {
		r.systemMu.Lock()
		locations := r.systemMap.FindSystemFonts(name)
		r.systemMu.Unlock()
		if len(locations) > 0 {
			return append([]fontscan.Location(nil), locations...), nil
		}
	}

	if r.scanCatalog != nil {
		r.catalogOnce.Do(func() {
			r.catalog, r.catalogErr = r.scanCatalog()
		})
	}
	locations := r.catalog[font.NormalizeFamily(name)]
	if len(locations) > 0 {
		return append([]fontscan.Location(nil), locations...), nil
	}
	if r.catalogErr != nil {
		return nil, r.catalogErr
	}
	return nil, r.systemErr
}

func sortFontLocationsByRegularPreference(locations []fontscan.Location, describe func(fontscan.Location) (font.Description, bool)) {
	sort.SliceStable(locations, func(i, j int) bool {
		leftDesc, leftOK := describe(locations[i])
		rightDesc, rightOK := describe(locations[j])
		if leftOK != rightOK {
			return leftOK
		}
		if !leftOK {
			return false
		}
		leftScore := regularFontScore(leftDesc.Aspect)
		rightScore := regularFontScore(rightDesc.Aspect)
		return leftScore < rightScore
	})
}

func regularFontScore(aspect font.Aspect) float32 {
	score := fontWeightDistance(aspect.Weight, font.WeightNormal)
	if aspect.Style != font.StyleNormal {
		score += 1000
	}
	score += fontStretchDistance(aspect.Stretch, font.StretchNormal) * 100
	return score
}

func fontWeightDistance(value, target font.Weight) float32 {
	diff := float32(value - target)
	if diff < 0 {
		return -diff
	}
	return diff
}

func fontStretchDistance(value, target font.Stretch) float32 {
	diff := float32(value - target)
	if diff < 0 {
		return -diff
	}
	return diff
}

func describeFontLocation(location fontscan.Location) (font.Description, bool) {
	file, err := os.Open(location.File)
	if err != nil {
		return font.Description{}, false
	}
	defer file.Close()

	loaders, err := ot.NewLoaders(file)
	if err != nil {
		return font.Description{}, false
	}
	if int(location.Index) >= len(loaders) {
		return font.Description{}, false
	}
	desc, _ := font.Describe(loaders[location.Index], nil)
	return desc, true
}

func scanFontLocationsByName(name string) []fontscan.Location {
	catalog, err := scanSystemFontCatalog()
	if err != nil {
		return nil
	}
	return append([]fontscan.Location(nil), catalog[font.NormalizeFamily(name)]...)
}

func scanSystemFontCatalog() (fontCatalog, error) {
	dirs, err := fontscan.DefaultFontDirectories(fontResolverLogger{})
	if err != nil {
		return nil, err
	}

	seenDirs := make(map[string]bool)
	seenFiles := make(map[string]bool)
	catalog := make(fontCatalog)
	for _, dir := range dirs {
		for _, scanDir := range fontScanDirectoryCandidates(dir) {
			if seenDirs[scanDir] {
				continue
			}
			seenDirs[scanDir] = true
			scanDirectoryForFontCatalog(scanDir, seenFiles, catalog)
		}
	}
	return catalog, nil
}

func fontScanDirectoryCandidates(dir string) []string {
	out := []string{dir}
	resolved, err := filepath.EvalSymlinks(dir)
	if err == nil && resolved != "" && resolved != dir {
		out = append(out, resolved)
	}
	return out
}

func scanDirectoryForFontCatalog(dir string, seenFiles map[string]bool, catalog fontCatalog) {
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

		for _, described := range describeFontFile(path) {
			family := font.NormalizeFamily(described.description.Family)
			if family == "" {
				continue
			}
			catalog[family] = append(catalog[family], fontscan.Location{File: path, Index: described.index})
		}
		return nil
	})
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func describeFontFile(path string) []describedFont {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	loaders, err := ot.NewLoaders(file)
	if err != nil {
		return nil
	}

	descriptions := make([]describedFont, 0, len(loaders))
	var buffer []byte
	for index, loader := range loaders {
		desc, nextBuffer := font.Describe(loader, buffer)
		buffer = nextBuffer
		descriptions = append(descriptions, describedFont{index: uint16(index), description: desc})
	}
	return descriptions
}

func loadFontResourceFromLocation(name string, location fontscan.Location) (fyne.Resource, string, error) {
	path := location.File
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttc", ".otc":
		extracted, err := extractCollectionFont(path, location.Index, name)
		if err != nil {
			return nil, "", err
		}
		// extractCollectionFont fully validates newly generated cache entries.
		// Existing entries are keyed by source metadata and checked structurally,
		// so reparsing every glyph on every launch is unnecessary.
		res, err := loadCachedFontResourceFromPath(extracted)
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

func loadCachedFontResourceFromPath(path string) (fyne.Resource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := validateCachedFontData(data); err != nil {
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

// validateCachedFontData checks the SFNT header and table bounds without
// decoding glyph outlines. Collection cache names include the source path,
// size, and modification time, and writes use rename, so this structural check
// is sufficient to reject truncated or obviously corrupt cache files.
func validateCachedFontData(data []byte) error {
	return validateCachedFontDirectory(data, uint64(len(data)))
}

func validateCachedFontFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}

	header := make([]byte, 12)
	if _, err := io.ReadFull(file, header); err != nil {
		return err
	}
	tableCount := int(binary.BigEndian.Uint16(header[4:6]))
	if tableCount == 0 || info.Size() < 12 || int64(tableCount) > (info.Size()-12)/16 {
		return fmt.Errorf("invalid font table count %d", tableCount)
	}
	directory := make([]byte, 12+tableCount*16)
	copy(directory, header)
	if _, err := io.ReadFull(file, directory[12:]); err != nil {
		return err
	}
	return validateCachedFontDirectory(directory, uint64(info.Size()))
}

func validateCachedFontDirectory(data []byte, fileSize uint64) error {
	if len(data) < 12 {
		return fmt.Errorf("font data too short")
	}
	signature := binary.BigEndian.Uint32(data[:4])
	switch signature {
	case 0x00010000, 0x4f54544f, 0x74727565, 0x74797031: // TrueType, OTTO, true, typ1
	default:
		return fmt.Errorf("unsupported font signature %#x", signature)
	}

	tableCount := int(binary.BigEndian.Uint16(data[4:6]))
	if tableCount == 0 || tableCount > (len(data)-12)/16 {
		return fmt.Errorf("invalid font table count %d", tableCount)
	}
	for i := 0; i < tableCount; i++ {
		record := 12 + i*16
		offset := uint64(binary.BigEndian.Uint32(data[record+8 : record+12]))
		length := uint64(binary.BigEndian.Uint32(data[record+12 : record+16]))
		if offset+length < offset || offset+length > fileSize {
			return fmt.Errorf("font table %d is out of bounds", i)
		}
	}
	return nil
}

func extractCollectionFont(path string, index uint16, family string) (string, error) {
	cachePath, err := collectionCachePath(path, index, family)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(cachePath); err == nil {
		if err := validateCachedFontFile(cachePath); err == nil {
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
