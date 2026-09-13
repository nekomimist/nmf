package jobs

import (
	"errors"
	"os"
	"strings"

	"nmf/internal/fileinfo"
)

// anchorExtractionRoot retains the actual extraction directory even when its
// parent is renamed, and confines every subsequent local write beneath it.
func anchorExtractionRoot(root executionPath) (executionPath, error) {
	before, err := root.localRoot.Lstat(root.path)
	if err != nil {
		return root, err
	}
	if !before.IsDir() || fileinfo.IsLinkModeCandidate(before.Mode()) {
		return root, errors.New("extraction root is not a plain directory")
	}
	anchor, err := root.localRoot.OpenRoot(root.path)
	if err != nil {
		return root, err
	}
	after, err := anchor.Stat(".")
	if err != nil || !os.SameFile(before, after) {
		anchor.Close()
		return root, errors.New("extraction root changed while opening")
	}
	root.rootDisplay = root.displayPath()
	root.path = "."
	root.localRoot = anchor
	return root, nil
}

// Direct SMB exposes path operations rather than a confined directory handle.
// Check each existing component and create missing parents one at a time. This
// rejects existing links but cannot make remote component replacement atomic.
func ensureSMBArchiveParents(execCtx *executionContext, root, parent executionPath) error {
	if root.backend != backendSMB {
		return nil
	}
	rel := strings.TrimPrefix(parent.path, strings.TrimRight(root.path, "/")+"/")
	current := root
	paths := []executionPath{root}
	if parent.path != root.path {
		for _, part := range strings.Split(rel, "/") {
			if err := fileinfo.ValidateArchiveEntryBaseName(part); err != nil {
				return err
			}
			current = joinPath(current, part)
			paths = append(paths, current)
		}
	}
	for _, p := range paths {
		info, err := lstatPath(execCtx, p)
		if fileinfo.IsNotExist(err) {
			ops, openErr := execCtx.smbOpsFor(p)
			if openErr != nil {
				return openErr
			}
			if mkErr := ops.Mkdir(p.path, 0755); mkErr != nil && !os.IsExist(mkErr) {
				return wrapPath(p.displayPath(), mkErr)
			}
			info, err = lstatPath(execCtx, p)
		}
		if err != nil {
			return wrapPath(p.displayPath(), err)
		}
		if !info.IsDir() || isLinkLikeForTraversal(execCtx, p, info) {
			return wrapPath(p.displayPath(), errors.New("archive destination component is not a plain directory"))
		}
	}
	return nil
}
