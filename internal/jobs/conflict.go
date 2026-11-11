package jobs

import (
	"fmt"
	"os"
	"time"

	"nmf/internal/fileinfo"
)

func resolveDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo os.FileInfo) (executionPath, bool, bool, error) {
	if sameExecutionPath(src, dst) {
		if j.Type == TypeMove {
			return dst, false, false, nil
		}
		return askDestinationConflict(j, execCtx, src, dst, srcInfo, srcInfo)
	}

	dstInfo, err := lstatPath(execCtx, dst)
	if err != nil {
		if fileinfo.IsNotExist(err) {
			return dst, false, false, nil
		}
		return dst, false, false, wrapPath(dst.displayPath(), err)
	}
	if srcInfo.IsDir() && dstInfo.IsDir() {
		return dst, false, false, nil
	}
	return askDestinationConflict(j, execCtx, src, dst, srcInfo, dstInfo)
}

func askDestinationConflict(j *Job, execCtx *executionContext, src, dst executionPath, srcInfo, dstInfo os.FileInfo) (executionPath, bool, bool, error) {
	for {
		suggested, err := nextAvailablePath(execCtx, dst)
		if err != nil {
			return dst, false, false, err
		}
		resolution := destinationConflictResolution(j, src, dst, suggested, srcInfo, dstInfo)
		rememberConflictResolution(j, resolution)
		if resolution.Action != ConflictRename {
			return applyDestinationConflict(resolution.Action, dst, suggested, srcInfo, dstInfo)
		}

		renamed, retry, err := renameConflictDestination(j, execCtx, dst, resolution.NewName)
		if err != nil || !retry {
			return renamed, false, false, err
		}
		dst = renamed
		j.conflictDefault = ConflictRename
	}
}

func destinationConflictResolution(j *Job, src, dst, suggested executionPath, srcInfo, dstInfo os.FileInfo) ConflictResolution {
	switch j.conflictDefault {
	case ConflictSkip, ConflictAutoSuffix, ConflictOverwriteIfNewer, ConflictOverwrite:
		return ConflictResolution{Action: j.conflictDefault}
	}
	defaultAction := j.conflictDefault
	if defaultAction == "" {
		defaultAction = ConflictOverwriteIfNewer
	}
	return resolveConflict(j, ConflictRequest{
		JobID:          j.ID,
		Type:           j.Type,
		SourcePath:     src.displayPath(),
		Destination:    dst.displayPath(),
		SourceModified: srcInfo.ModTime(),
		DestModified:   dstInfo.ModTime(),
		SuggestedName:  baseName(suggested),
		SuggestedPath:  suggested.displayPath(),
		IsDir:          srcInfo.IsDir(),
		DefaultAction:  defaultAction,
		CanApplyToRest: true,
	})
}

func rememberConflictResolution(j *Job, resolution ConflictResolution) {
	if !resolution.ApplyToRest {
		return
	}
	switch resolution.Action {
	case ConflictSkip, ConflictOverwriteIfNewer, ConflictOverwrite, ConflictRename, ConflictAutoSuffix:
		j.conflictDefault = resolution.Action
	case "":
		j.conflictDefault = ConflictAutoSuffix
	}
}

func applyDestinationConflict(action ConflictAction, dst, suggested executionPath, srcInfo, dstInfo os.FileInfo) (executionPath, bool, bool, error) {
	switch action {
	case ConflictSkip:
		return dst, true, false, nil
	case ConflictCancelJob:
		return dst, false, false, errCanceled
	case ConflictOverwriteIfNewer:
		overwrite := canOverwriteConflict(srcInfo, dstInfo) && sourceClearlyNewer(srcInfo, dstInfo)
		return dst, !overwrite, overwrite, nil
	case ConflictOverwrite:
		overwrite := canOverwriteConflict(srcInfo, dstInfo)
		return dst, !overwrite, overwrite, nil
	case ConflictAutoSuffix, "":
		return suggested, false, false, nil
	default:
		return dst, false, false, fmt.Errorf("unknown conflict action: %s", action)
	}
}

// renameConflictDestination requests another prompt for invalid or occupied
// names. Without a resolver, invalid names must fail instead of looping.
func renameConflictDestination(j *Job, execCtx *executionContext, dst executionPath, newName string) (renamed executionPath, retry bool, err error) {
	name, err := fileinfo.ValidateRenameName(newName)
	if err != nil {
		if j.Resolver == nil {
			return dst, false, wrapPath(dst.displayPath(), err)
		}
		return dst, true, nil
	}
	renamed = joinPath(dirPath(dst), name)
	exists, err := pathExists(execCtx, renamed)
	if err != nil {
		return renamed, false, wrapPath(renamed.displayPath(), err)
	}
	return renamed, exists, nil
}

func canOverwriteConflict(srcInfo, dstInfo os.FileInfo) bool {
	if srcInfo == nil || dstInfo == nil {
		return false
	}
	if srcInfo.IsDir() || dstInfo.IsDir() {
		return false
	}
	return srcInfo.Mode().Type() == dstInfo.Mode().Type()
}

func sourceClearlyNewer(srcInfo, dstInfo os.FileInfo) bool {
	const fatTimestampResolution = 2 * time.Second
	return srcInfo.ModTime().After(dstInfo.ModTime().Add(fatTimestampResolution))
}

func resolveConflict(j *Job, req ConflictRequest) ConflictResolution {
	if j.Resolver == nil {
		return ConflictResolution{Action: ConflictAutoSuffix}
	}
	return j.Resolver(j.ctx, req)
}
