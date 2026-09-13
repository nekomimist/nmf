package filecompare

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"nmf/internal/fileinfo"
)

// Method identifies how source files are matched against destination files.
type Method string

const (
	MissingOrNewer   Method = "missing_or_newer"
	Missing          Method = "missing"
	Newer            Method = "newer"
	SizeEqual        Method = "size_equal"
	SizeTimeEqual    Method = "size_time_equal"
	SizeContentEqual Method = "size_content_equal"
)

// Result describes the source files matched by a comparison.
type Result struct {
	Matched     []fileinfo.FileInfo
	SourceCount int
	TargetCount int
	ErrorCount  int
	FirstError  error
}

// CompareDirectFiles compares non-directory source files against targetDir's
// non-directory direct children by exact file name.
func CompareDirectFiles(sourceFiles []fileinfo.FileInfo, targetDir string, method Method) (Result, error) {
	return CompareDirectFilesContext(context.Background(), sourceFiles, targetDir, method)
}

// CompareDirectFilesContext compares files until completion or cancellation.
func CompareDirectFilesContext(ctx context.Context, sourceFiles []fileinfo.FileInfo, targetDir string, method Method) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	targetEntries, err := fileinfo.ReadDirPortableContext(ctx, targetDir)
	if err != nil {
		return Result{}, err
	}

	targets := make(map[string]fileinfo.FileInfo, len(targetEntries))
	for _, entry := range targetEntries {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fi, err := fileinfo.FileInfoFromDirEntry(targetDir, entry)
		if err != nil || !isComparableFile(fi) {
			continue
		}
		targets[fi.Name] = fi
	}

	result := Result{TargetCount: len(targets)}
	for _, source := range sourceFiles {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if !isComparableFile(source) {
			continue
		}
		result.SourceCount++
		target, exists := targets[source.Name]
		matched, err := matches(ctx, source, target, exists, method)
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

func isComparableFile(fi fileinfo.FileInfo) bool {
	return fi.Name != ".." && !fi.IsDir && fi.Status != fileinfo.StatusDeleted
}

func matches(ctx context.Context, source, target fileinfo.FileInfo, exists bool, method Method) (bool, error) {
	switch method {
	case MissingOrNewer:
		return !exists || source.Modified.After(target.Modified), nil
	case Missing:
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
		equal, err := contentEqual(ctx, source.Path, target.Path)
		if err != nil {
			return false, err
		}
		return equal, nil
	default:
		return false, fmt.Errorf("unknown compare method: %s", method)
	}
}

func contentEqual(ctx context.Context, leftPath, rightPath string) (bool, error) {
	left, err := fileinfo.OpenPortableContext(ctx, leftPath)
	if err != nil {
		return false, fmt.Errorf("%s: %w", leftPath, err)
	}
	defer left.Close()

	right, err := fileinfo.OpenPortableContext(ctx, rightPath)
	if err != nil {
		return false, fmt.Errorf("%s: %w", rightPath, err)
	}
	defer right.Close()

	return readersEqual(ctx, left, right)
}

func readersEqual(ctx context.Context, left, right io.Reader) (bool, error) {
	left = &comparisonReader{ctx: ctx, reader: left}
	right = &comparisonReader{ctx: ctx, reader: right}
	leftBuf := make([]byte, 32*1024)
	rightBuf := make([]byte, 32*1024)
	for {
		leftN, leftErr := io.ReadFull(left, leftBuf)
		rightN, rightErr := io.ReadFull(right, rightBuf)
		if leftErr == io.ErrUnexpectedEOF {
			leftErr = io.EOF
		}
		if rightErr == io.ErrUnexpectedEOF {
			rightErr = io.EOF
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if leftN != rightN || !bytes.Equal(leftBuf[:leftN], rightBuf[:rightN]) {
			return false, nil
		}
		if leftErr == io.EOF && rightErr == io.EOF {
			return true, nil
		}
		if leftErr != nil && leftErr != io.EOF {
			return false, leftErr
		}
		if rightErr != nil && rightErr != io.EOF {
			return false, rightErr
		}
	}
}

type comparisonReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *comparisonReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
