package fileinfo

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
)

func TestArchiveVFSReadDirStatAndOpen(t *testing.T) {
	archivePath := writeTestZip(t, map[string]string{
		"docs/readme.txt": "hello archive",
		"root.txt":        "at root",
	})

	if !IsSupportedArchive(archivePath) {
		t.Fatalf("IsSupportedArchive(%q) = false, want true", archivePath)
	}

	root := ArchiveRootPath(archivePath)
	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 2 {
		t.Fatalf("root entries len = %d, want 2", len(entries))
	}

	innerPath := JoinPath(root, "docs/readme.txt")
	info, err := StatPortable(innerPath)
	if err != nil {
		t.Fatalf("StatPortable(%q) returned error: %v", innerPath, err)
	}
	if info.IsDir() || info.Size() != int64(len("hello archive")) {
		t.Fatalf("unexpected info: isDir=%t size=%d", info.IsDir(), info.Size())
	}

	vfs, parsed, err := ResolveRead(innerPath)
	if err != nil {
		t.Fatalf("ResolveRead(%q) returned error: %v", innerPath, err)
	}
	if parsed.Scheme != SchemeArchive || parsed.Native != "docs/readme.txt" {
		t.Fatalf("unexpected parsed archive path: %+v", parsed)
	}
	rc, err := vfs.Open(parsed.Native)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", parsed.Native, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "hello archive" {
		t.Fatalf("archive file content = %q, want %q", string(data), "hello archive")
	}
}

func TestReadDirPortableContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := ReadDirPortableContext(ctx, t.TempDir())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReadDirPortableContext canceled error = %v, want context.Canceled", err)
	}
}

func TestArchiveVFSCloseRemovesTemporarySourceIdempotently(t *testing.T) {
	temp, err := os.CreateTemp(t.TempDir(), "archive-source-*")
	if err != nil {
		t.Fatal(err)
	}
	path := temp.Name()
	if err := temp.Close(); err != nil {
		t.Fatal(err)
	}
	vfs := &ArchiveVFS{tempPath: path}

	if err := vfs.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("temporary source still exists or stat failed: %v", err)
	}
	if err := vfs.Close(); err != nil {
		t.Fatalf("second Close should be idempotent: %v", err)
	}
}

func TestArchiveVFSShiftJISZipNames(t *testing.T) {
	archivePath := writeShiftJISZip(t)
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "shift_jis"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "てすと" {
		t.Fatalf("root entries = %v, want てすと", entryNames(entries))
	}

	innerPath := JoinPath(root, "てすと/UNICODE Text-test2006.txt")
	vfs, parsed, err := ResolveRead(innerPath)
	if err != nil {
		t.Fatalf("ResolveRead(%q) returned error: %v", innerPath, err)
	}
	rc, err := vfs.Open(parsed.Native)
	if err != nil {
		t.Fatalf("Open(%q) returned error: %v", parsed.Native, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if string(data) != "hello zip\n" {
		t.Fatalf("archive file content = %q, want %q", string(data), "hello zip\n")
	}
}

func TestArchiveVFSConfiguredCP437ZipNames(t *testing.T) {
	archivePath := writeEncodedZip(t, charmap.CodePage437, map[string]string{
		"café/":           "",
		"café/readme.txt": "hello cp437\n",
	})
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "cp437"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "café" {
		t.Fatalf("root entries = %v, want café", entryNames(entries))
	}
}

func TestArchiveVFSKeepsValidUTF8ZipNamesWithoutUTF8Flag(t *testing.T) {
	archivePath := writeNonUTF8FlaggedZip(t, map[string]string{
		"てすと/":                          "",
		"てすと/UNICODE Text-test2006.txt": "hello utf8\n",
	})
	setArchiveOptionsForTest(t, ArchiveOptions{ZipNameEncoding: "shift_jis"})
	root := ArchiveRootPath(archivePath)

	entries, err := ReadDirPortable(root)
	if err != nil {
		t.Fatalf("ReadDirPortable(%q) returned error: %v", root, err)
	}
	if len(entries) != 1 || entries[0].Name() != "てすと" {
		t.Fatalf("root entries = %v, want てすと", entryNames(entries))
	}
}

func TestArchiveVFSRejectsUnsafeEntryNames(t *testing.T) {
	tests := []string{
		"../evil.txt",
		"dir/../../evil.txt",
		"/abs.txt",
		"C:/evil.txt",
		`dir\evil.txt`,
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			archivePath := writeTestZip(t, map[string]string{name: "bad"})
			_, err := ReadDirPortable(ArchiveRootPath(archivePath))
			if !errors.Is(err, ErrUnsafeArchiveEntry) {
				t.Fatalf("ReadDirPortable unsafe archive error = %v, want ErrUnsafeArchiveEntry", err)
			}
		})
	}
}

func TestArchiveVFSPromptsForEncryptedSevenZip(t *testing.T) {
	provider := setArchivePasswordProviderForTest(t, "secret")
	archivePath := filepath.Join("testdata", "encrypted.7z")

	entries, err := ReadDirPortable(ArchiveRootPath(archivePath))
	if err != nil {
		t.Fatalf("ReadDirPortable encrypted 7z returned error: %v", err)
	}
	if got := entryNames(entries); len(got) != 1 || got[0] != "secret.txt" {
		t.Fatalf("encrypted 7z entries = %v, want [secret.txt]", got)
	}

	entryPath := JoinPath(ArchiveRootPath(archivePath), "secret.txt")
	vfs, parsed, err := ResolveRead(entryPath)
	if err != nil {
		t.Fatalf("ResolveRead encrypted 7z returned error: %v", err)
	}
	rc, err := vfs.Open(parsed.Native)
	if err != nil {
		t.Fatalf("OpenPortable encrypted 7z returned error: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll encrypted 7z returned error: %v", err)
	}
	if string(data) != "secret archive data\n" {
		t.Fatalf("encrypted 7z content = %q", string(data))
	}
	if provider.callsFor(archivePath) != 1 {
		t.Fatalf("encrypted 7z prompt count = %d, want 1", provider.callsFor(archivePath))
	}
}

func TestArchiveVFSPromptsForEncryptedRar(t *testing.T) {
	provider := setArchivePasswordProviderForTest(t, "secret")
	archivePath := filepath.Join("testdata", "encrypted.rar")

	entries, err := ReadDirPortable(ArchiveRootPath(archivePath))
	if err != nil {
		t.Fatalf("ReadDirPortable encrypted rar returned error: %v", err)
	}
	if got := entryNames(entries); len(got) != 1 || got[0] != "secret.txt" {
		t.Fatalf("encrypted rar entries = %v, want [secret.txt]", got)
	}
	if provider.callsFor(archivePath) != 1 {
		t.Fatalf("encrypted rar prompt count = %d, want 1", provider.callsFor(archivePath))
	}
}

func TestArchiveVFSPromptsForContentEncryptedVisibleNames(t *testing.T) {
	tests := []struct {
		name        string
		archivePath string
	}{
		{name: "7z", archivePath: filepath.Join("testdata", "encrypted-names-visible.7z")},
		{name: "rar", archivePath: filepath.Join("testdata", "encrypted-names-visible.rar")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setArchivePasswordProviderForTest(t, "secret")

			entries, err := ReadDirPortable(ArchiveRootPath(tt.archivePath))
			if err != nil {
				t.Fatalf("ReadDirPortable content-encrypted %s returned error: %v", tt.name, err)
			}
			if got := entryNames(entries); len(got) != 1 || got[0] != "secret.txt" {
				t.Fatalf("content-encrypted %s entries = %v, want [secret.txt]", tt.name, got)
			}

			entryPath := JoinPath(ArchiveRootPath(tt.archivePath), "secret.txt")
			vfs, parsed, err := ResolveRead(entryPath)
			if err != nil {
				t.Fatalf("ResolveRead content-encrypted %s returned error: %v", tt.name, err)
			}
			rc, err := vfs.Open(parsed.Native)
			if err != nil {
				t.Fatalf("Open content-encrypted %s returned error: %v", tt.name, err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll content-encrypted %s returned error: %v", tt.name, err)
			}
			if string(data) != "secret archive data\n" {
				t.Fatalf("content-encrypted %s content = %q", tt.name, string(data))
			}
			if provider.callsFor(tt.archivePath) != 1 {
				t.Fatalf("content-encrypted %s prompt count = %d, want 1", tt.name, provider.callsFor(tt.archivePath))
			}
		})
	}
}

func TestExtractArchivePromptsForEncryptedArchives(t *testing.T) {
	tests := []struct {
		name        string
		archivePath string
	}{
		{name: "7z", archivePath: filepath.Join("testdata", "encrypted.7z")},
		{name: "rar", archivePath: filepath.Join("testdata", "encrypted.rar")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := setArchivePasswordProviderForTest(t, "secret")
			got := make(map[string]string)

			err := ExtractArchive(t.Context(), tt.archivePath, func(_ context.Context, entry ArchiveEntry) error {
				if entry.Info.IsDir() {
					return nil
				}
				rc, err := entry.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				data, err := io.ReadAll(rc)
				if err != nil {
					return err
				}
				got[entry.Name] = string(data)
				return nil
			})
			if err != nil {
				t.Fatalf("ExtractArchive encrypted %s returned error: %v", tt.name, err)
			}
			if got["secret.txt"] != "secret archive data\n" {
				t.Fatalf("ExtractArchive encrypted %s content = %#v", tt.name, got)
			}
			if provider.callsFor(tt.archivePath) != 1 {
				t.Fatalf("ExtractArchive encrypted %s prompt count = %d, want 1", tt.name, provider.callsFor(tt.archivePath))
			}
		})
	}
}

func TestArchivePasswordProviderRetriesWrongPassword(t *testing.T) {
	provider := setArchivePasswordProviderForTest(t, "wrong", "secret")
	archivePath := filepath.Join("testdata", "encrypted.7z")

	_, err := ReadDirPortable(ArchiveRootPath(archivePath))
	if err != nil {
		t.Fatalf("ReadDirPortable retry encrypted 7z returned error: %v", err)
	}
	if provider.callsFor(archivePath) != 2 {
		t.Fatalf("encrypted 7z retry prompt count = %d, want 2", provider.callsFor(archivePath))
	}
	if !provider.sawRetry(archivePath) {
		t.Fatalf("encrypted 7z provider did not receive retry request")
	}
}

func TestArchivePasswordProviderCachesPassword(t *testing.T) {
	provider := setArchivePasswordProviderForTest(t, "secret")
	archivePath := filepath.Join("testdata", "encrypted.7z")

	if _, err := ReadDirPortable(ArchiveRootPath(archivePath)); err != nil {
		t.Fatalf("first ReadDirPortable encrypted 7z returned error: %v", err)
	}
	if _, err := ReadDirPortable(ArchiveRootPath(archivePath)); err != nil {
		t.Fatalf("second ReadDirPortable encrypted 7z returned error: %v", err)
	}
	if provider.callsFor(archivePath) != 1 {
		t.Fatalf("encrypted 7z cached prompt count = %d, want 1", provider.callsFor(archivePath))
	}
}

func TestArchivePasswordProviderCancel(t *testing.T) {
	setArchivePasswordProviderForTest(t)
	archivePath := filepath.Join("testdata", "encrypted.7z")

	_, err := ReadDirPortable(ArchiveRootPath(archivePath))
	if !errors.Is(err, ErrArchivePasswordRequired) {
		t.Fatalf("ReadDirPortable cancelled password error = %v, want ErrArchivePasswordRequired", err)
	}
}

func TestArchiveVFSDoesNotPromptForPlainZip(t *testing.T) {
	provider := setArchivePasswordProviderForTest(t, "secret")
	archivePath := writeTestZip(t, map[string]string{"plain.txt": "plain"})

	if _, err := ReadDirPortable(ArchiveRootPath(archivePath)); err != nil {
		t.Fatalf("ReadDirPortable plain zip returned error: %v", err)
	}
	if provider.totalCalls() != 0 {
		t.Fatalf("plain zip prompt count = %d, want 0", provider.totalCalls())
	}
}

func TestArchiveVFSAllowsDotPrefixedSafeEntryNames(t *testing.T) {
	archivePath := writeTestZip(t, map[string]string{
		"./dir/file.txt": "safe",
	})

	entries, err := ReadDirPortable(ArchiveRootPath(archivePath))
	if err != nil {
		t.Fatalf("ReadDirPortable returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "dir" {
		t.Fatalf("root entries = %v, want dir", entryNames(entries))
	}
}

func TestValidateArchiveEntryPathRejectsNUL(t *testing.T) {
	if err := ValidateArchiveEntryPath("bad\x00name.txt", false); !errors.Is(err, ErrUnsafeArchiveEntry) {
		t.Fatalf("ValidateArchiveEntryPath NUL error = %v, want ErrUnsafeArchiveEntry", err)
	}
}

func TestResolveArchiveZipNameEncoding(t *testing.T) {
	tests := []string{"", "shift_jis", "sjis", "cp932", "cp437", "ibm437", "utf-8", "UTF8"}
	for _, tt := range tests {
		if _, err := ResolveArchiveZipNameEncoding(tt); err != nil {
			t.Fatalf("ResolveArchiveZipNameEncoding(%q) returned error: %v", tt, err)
		}
	}
	if _, err := ResolveArchiveZipNameEncoding("not-a-real-encoding"); err == nil {
		t.Fatal("ResolveArchiveZipNameEncoding should reject unknown encoding")
	}
}

func TestArchivePathHelpers(t *testing.T) {
	root := ArchiveRootPath("/tmp/sample.zip")
	if root != "/tmp/sample.zip!/" {
		t.Fatalf("ArchiveRootPath got %q", root)
	}
	inner := JoinPath(root, "dir/file.txt")
	if inner != "/tmp/sample.zip!/dir/file.txt" {
		t.Fatalf("JoinPath archive got %q", inner)
	}
	if parent := ParentPath(inner); parent != "/tmp/sample.zip!/dir" {
		t.Fatalf("ParentPath archive got %q", parent)
	}
	if parent := ParentPath(root); parent != "/tmp" {
		t.Fatalf("ParentPath archive root got %q", parent)
	}
	if base := BaseName(root); base != "sample.zip" {
		t.Fatalf("BaseName archive root got %q", base)
	}
}

func writeTestZip(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "sample.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("Create(%q) returned error: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("Write(%q) returned error: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close returned error: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("file Close returned error: %v", err)
	}
	return archivePath
}

func writeShiftJISZip(t *testing.T) string {
	return writeEncodedZip(t, japanese.ShiftJIS, map[string]string{
		"てすと/":                          "",
		"てすと/UNICODE Text-test2006.txt": "hello zip\n",
	})
}

func writeEncodedZip(t *testing.T, enc encoding.Encoding, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "encoded.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		encodedName, err := enc.NewEncoder().String(name)
		if err != nil {
			t.Fatalf("encode %q returned error: %v", name, err)
		}
		hdr := &zip.FileHeader{Name: encodedName, NonUTF8: true, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("CreateHeader(%q) returned error: %v", name, err)
		}
		if content != "" {
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("Write(%q) returned error: %v", name, err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close returned error: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("file Close returned error: %v", err)
	}
	return archivePath
}

func writeNonUTF8FlaggedZip(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "utf8-no-flag.zip")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create zip returned error: %v", err)
	}
	zw := zip.NewWriter(out)
	for name, content := range files {
		hdr := &zip.FileHeader{Name: name, NonUTF8: true, Method: zip.Store}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatalf("CreateHeader(%q) returned error: %v", name, err)
		}
		if content != "" {
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatalf("Write(%q) returned error: %v", name, err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close returned error: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("file Close returned error: %v", err)
	}
	return archivePath
}

func setArchiveOptionsForTest(t *testing.T, opts ArchiveOptions) {
	t.Helper()
	previous := currentArchiveOptions()
	if err := SetArchiveOptions(opts); err != nil {
		t.Fatalf("SetArchiveOptions(%+v) returned error: %v", opts, err)
	}
	t.Cleanup(func() {
		if err := SetArchiveOptions(previous); err != nil {
			t.Fatalf("restore archive options returned error: %v", err)
		}
	})
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

type fakeArchivePasswordProvider struct {
	mu      sync.Mutex
	answers []string
	calls   map[string]int
	retried map[string]bool
}

func setArchivePasswordProviderForTest(t *testing.T, answers ...string) *fakeArchivePasswordProvider {
	t.Helper()
	previous := archivePasswordProvider
	provider := &fakeArchivePasswordProvider{
		answers: append([]string(nil), answers...),
		calls:   make(map[string]int),
		retried: make(map[string]bool),
	}
	SetArchivePasswordProvider(NewCachedArchivePasswordProvider(provider))
	t.Cleanup(func() {
		SetArchivePasswordProvider(previous)
	})
	return provider
}

func (p *fakeArchivePasswordProvider) GetArchivePassword(_ context.Context, req ArchivePasswordRequest) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls[req.ArchivePath]++
	if req.Retry {
		p.retried[req.ArchivePath] = true
	}
	if len(p.answers) == 0 {
		return "", errors.New("cancelled")
	}
	answer := p.answers[0]
	p.answers = p.answers[1:]
	return answer, nil
}

func (p *fakeArchivePasswordProvider) callsFor(path string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls[path]
}

func (p *fakeArchivePasswordProvider) totalCalls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	total := 0
	for _, n := range p.calls {
		total += n
	}
	return total
}

func (p *fakeArchivePasswordProvider) sawRetry(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.retried[path]
}
