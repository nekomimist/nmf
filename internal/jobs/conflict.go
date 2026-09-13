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
		var resolution ConflictResolution
		switch j.conflictDefault {
		case ConflictSkip:
			resolution = ConflictResolution{Action: ConflictSkip}
		case ConflictAutoSuffix:
			resolution = ConflictResolution{Action: ConflictAutoSuffix}
		case ConflictOverwriteIfNewer:
			resolution = ConflictResolution{Action: ConflictOverwriteIfNewer}
		case ConflictOverwrite:
			resolution = ConflictResolution{Action: ConflictOverwrite}
		default:
			defaultAction := j.conflictDefault
			if defaultAction == "" {
				defaultAction = ConflictOverwriteIfNewer
			}
			resolution = resolveConflict(j, ConflictRequest{
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

		switch resolution.Action {
		case ConflictSkip:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictSkip
			}
			return dst, true, false, nil
		case ConflictCancelJob:
			return dst, false, false, errCanceled
		case ConflictOverwriteIfNewer:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictOverwriteIfNewer
			}
			if canOverwriteConflict(srcInfo, dstInfo) && sourceClearlyNewer(srcInfo, dstInfo) {
				return dst, false, true, nil
			}
			return dst, true, false, nil
		case ConflictOverwrite:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictOverwrite
			}
			if canOverwriteConflict(srcInfo, dstInfo) {
				return dst, false, true, nil
			}
			return dst, true, false, nil
		case ConflictRename:
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictRename
			}
			name, err := fileinfo.ValidateRenameName(resolution.NewName)
			if err != nil {
				if j.Resolver == nil {
					return dst, false, false, wrapPath(dst.displayPath(), err)
				}
				j.conflictDefault = ConflictRename
				continue
			}
			renamed := joinPath(dirPath(dst), name)
			if exists, err := pathExists(execCtx, renamed); err != nil {
				return renamed, false, false, wrapPath(renamed.displayPath(), err)
			} else if exists {
				dst = renamed
				j.conflictDefault = ConflictRename
				continue
			}
			return renamed, false, false, nil
		case ConflictAutoSuffix, "":
			if resolution.ApplyToRest {
				j.conflictDefault = ConflictAutoSuffix
			}
			return suggested, false, false, nil
		default:
			return dst, false, false, fmt.Errorf("unknown conflict action: %s", resolution.Action)
		}
	}
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
