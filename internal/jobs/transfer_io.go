package jobs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const progressNotifyInterval = 350 * time.Millisecond

// createTransferTemp exclusively creates an owned output beside the destination.
// Never reuse a predictable .part path: it may contain unrelated user data.
func createTransferTemp(execCtx *executionContext, dst executionPath, mode os.FileMode) (executionPath, io.ReadWriteCloser, error) {
	for range 100 {
		tmp := joinPath(dirPath(dst), ".nmf-"+rand.Text()+".part")
		out, err := createExclusivePath(execCtx, tmp, mode)
		if os.IsExist(err) {
			continue
		}
		return tmp, out, err
	}
	return executionPath{}, nil, fmt.Errorf("could not create temporary output in %s", dirPath(dst).displayPath())
}

// publishExclusiveCopy supports filesystems without no-replace rename (for
// example some mounted volumes). The destination may be visible while copied,
// but exclusive creation still protects a concurrent creator's data.
func publishExclusiveCopy(j *Job, execCtx *executionContext, tmp, dst executionPath) error {
	in, err := openReadPath(execCtx, tmp)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := lstatPath(execCtx, tmp)
	if err != nil {
		return err
	}
	out, err := createExclusivePath(execCtx, dst, info.Mode())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, &transferContextReader{ctx: j.ctx, reader: in})
	closeErr := out.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		_ = removePath(execCtx, dst)
		return err
	}
	return removePath(execCtx, tmp)
}

type transferContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *transferContextReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil {
		return 0, errCanceled
	}
	return r.reader.Read(p)
}

func copyFileWithCancel(j *Job, execCtx *executionContext, src, dst executionPath, fi os.FileInfo, overwrite bool) error {
	in, err := openReadPath(execCtx, src)
	if err != nil {
		return wrapPath(src.displayPath(), err)
	}
	defer in.Close()
	return copyReaderWithCancel(j, execCtx, in, src.displayPath(), dst, fi, overwrite)
}

func copyReaderWithCancel(j *Job, execCtx *executionContext, in io.Reader, srcDisplay string, dst executionPath, fi os.FileInfo, overwrite bool) error {
	if err := ensureDir(execCtx, dirPath(dst), 0755); err != nil {
		return wrapPath(dst.displayPath(), err)
	}

	tmp, out, err := createTransferTemp(execCtx, dst, fi.Mode())
	if err != nil {
		return wrapPath(tmp.displayPath(), err)
	}

	totalBytes := fi.Size()
	if totalBytes < 0 {
		totalBytes = 0
	}
	j.beginFileProgress(srcDisplay, totalBytes)

	if execCtx.copyBuffer == nil {
		execCtx.copyBuffer = make([]byte, 1<<20)
	}
	buf := execCtx.copyBuffer
	for {
		if canceled(j) {
			out.Close()
			_ = removePath(execCtx, tmp)
			return errCanceled
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				_ = removePath(execCtx, tmp)
				return wrapPath(tmp.displayPath(), werr)
			}
			j.addFileProgress(int64(n), false)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			_ = removePath(execCtx, tmp)
			return wrapPath(srcDisplay, rerr)
		}
	}
	j.completeFileProgress()
	if local, ok := out.(*os.File); ok {
		if err := local.Chmod(fi.Mode().Perm()); err != nil {
			out.Close()
			_ = removePath(execCtx, tmp)
			return wrapPath(tmp.displayPath(), err)
		}
	}
	if err := out.Close(); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(tmp.displayPath(), err)
	}

	dbg("job %d: rename %s -> %s", j.ID, tmp.displayPath(), dst.displayPath())
	if err := replacePath(j, execCtx, tmp, dst, overwrite); err != nil {
		_ = removePath(execCtx, tmp)
		return wrapPath(dst.displayPath(), err)
	}
	if shouldPreserveTimestamps(j) {
		if err := chtimesPath(execCtx, dst, fi.ModTime(), fi.ModTime()); err != nil {
			return wrapPath(dst.displayPath(), err)
		}
	}
	return nil
}
