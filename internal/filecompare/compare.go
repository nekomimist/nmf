package filecompare

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"nmf/internal/fileinfo"
)

// Method identifies which source entries to mark after comparing directories.
type Method string

const (
	MissingOrNewer      Method = "missing_or_newer"
	Missing             Method = "missing"
	Newer               Method = "newer"
	SizeEqual           Method = "size_equal"
	SizeTimeEqual       Method = "size_time_equal"
	SizeContentEqual    Method = "size_content_equal"
	MissingSubdirectory Method = "missing_subdirectory"
)

// Result describes the source entries matched by a comparison.
type Result struct {
	Matched     []fileinfo.FileInfo
	SourceCount int
	TargetCount int
	ErrorCount  int
	FirstError  error
}

// CompareDirectEntriesContext compares direct children by exact name.
// MissingSubdirectory compares directories. The other methods compare files.
func CompareDirectEntriesContext(ctx context.Context, sourceEntries []fileinfo.FileInfo, targetDir string, method Method) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	targetEntries, err := fileinfo.ReadDirPortableContext(ctx, targetDir)
	if err != nil {
		return Result{}, err
	}

	directories := method == MissingSubdirectory
	targets := make(map[string]fileinfo.FileInfo, len(targetEntries))
	targetErrors := make(map[string]error)
	for _, entry := range targetEntries {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fi, err := fileinfo.FileInfoFromDirEntry(targetDir, entry)
		if err != nil {
			targetErrors[entry.Name()] = err
			continue
		}
		if !fileinfo.IsFileOperationTarget(fi) || fi.IsDir != directories {
			continue
		}
		targets[fi.Name] = fi
	}

	result := Result{TargetCount: len(targets)}
	cmp := comparison{ctx: ctx}
	for _, source := range sourceEntries {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if !fileinfo.IsFileOperationTarget(source) || source.IsDir != directories {
			continue
		}
		result.SourceCount++
		target, exists := targets[source.Name]
		var matched bool
		err := targetErrors[source.Name]
		if err == nil {
			matched, err = cmp.matches(source, target, exists, method)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, ctxErr
		}
		if err != nil {
			result.ErrorCount++
			if result.FirstError == nil {
				result.FirstError = err
			}
			continue
		}
		if matched {
			result.Matched = append(result.Matched, source)
		}
	}
	return result, nil
}

type comparison struct {
	ctx    context.Context
	buffer []byte // allocated only for content comparisons, reused across file pairs
}

func (c *comparison) matches(source, target fileinfo.FileInfo, exists bool, method Method) (bool, error) {
	switch method {
	case MissingOrNewer:
		return !exists || source.Modified.After(target.Modified), nil
	case Missing, MissingSubdirectory:
		return !exists, nil
	case Newer:
		return exists && source.Modified.After(target.Modified), nil
	case SizeEqual:
		return exists && source.Size == target.Size, nil
	case SizeTimeEqual:
		return exists && source.Size == target.Size && source.Modified.Equal(target.Modified), nil
	case SizeContentEqual:
		if !exists || source.Size != target.Size {
			return false, nil
		}
		return c.contentEqual(source.Path, target.Path)
	default:
		return false, fmt.Errorf("unknown compare method: %s", method)
	}
}

func (c *comparison) contentEqual(leftPath, rightPath string) (bool, error) {
	left, err := fileinfo.OpenPortableContext(c.ctx, leftPath)
	if err != nil {
		return false, fmt.Errorf("%s: %w", leftPath, err)
	}
	defer left.Close()

	right, err := fileinfo.OpenPortableContext(c.ctx, rightPath)
	if err != nil {
		return false, fmt.Errorf("%s: %w", rightPath, err)
	}
	defer right.Close()

	return c.readersEqual(left, right)
}

func (c *comparison) readersEqual(left, right io.Reader) (bool, error) {
	const chunk = 32 * 1024
	if c.buffer == nil {
		c.buffer = make([]byte, 2*chunk)
	}
	leftBuf, rightBuf := c.buffer[:chunk], c.buffer[chunk:]
	for {
		leftN, leftErr := c.readChunk(left, leftBuf)
		if leftErr != nil && leftErr != io.EOF {
			return false, leftErr
		}
		rightN, rightErr := c.readChunk(right, rightBuf)
		if rightErr != nil && rightErr != io.EOF {
			return false, rightErr
		}
		if err := c.ctx.Err(); err != nil {
			return false, err
		}
		if leftN != rightN || !bytes.Equal(leftBuf[:leftN], rightBuf[:rightN]) {
			return false, nil
		}
		if leftErr != nil && rightErr != nil {
			return true, nil
		}
	}
}

// Preserve provider errors (including ErrUnexpectedEOF) even when a read
// returns data with the error. A short final chunk ending in EOF is normal.
func (c *comparison) readChunk(reader io.Reader, buf []byte) (int, error) {
	n := 0
	for n < len(buf) {
		if err := c.ctx.Err(); err != nil {
			return n, err
		}
		read, err := reader.Read(buf[n:])
		n += read
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
