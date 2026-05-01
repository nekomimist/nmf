package fileinfo

// OpenWithDefaultApp opens the given path with the OS-associated application.
// Archive entries are extracted to a temporary file first.
func OpenWithDefaultApp(p string) error {
	if IsArchivePath(p) {
		tmpPath, err := ExtractArchiveEntryToTemp(p)
		if err != nil {
			return err
		}
		return openNativeWithDefaultApp(tmpPath)
	}
	return openNativeWithDefaultApp(p)
}
