package main

import (
	"fmt"

	"nmf/internal/jobs"
	"nmf/internal/ui"
)

// ShowDeleteDialog confirms and queues trash or permanent deletion.
func (fm *FileManager) ShowDeleteDialog(permanent bool) {
	targets := fm.collectTargets()
	if len(targets) == 0 {
		debugPrint("FileManager: No valid target for delete")
		return
	}
	srcPaths := fm.collectTargetPaths()
	if len(srcPaths) == 0 {
		debugPrint("FileManager: No valid source paths for delete")
		return
	}

	dlg := ui.NewDeleteConfirmDialog(targets, permanent, fm.keyManager)
	dlg.ShowDialog(fm.window, func() {
		mode := jobs.DeleteModeTrash
		if permanent {
			mode = jobs.DeleteModePermanent
		}
		jobs.GetManager().EnqueueDelete(srcPaths, mode)
		if permanent {
			ui.ShowCompactMessageDialog(fm.window, "Delete", fmt.Sprintf("Queued permanent delete for %d item(s).", len(srcPaths)))
		} else {
			ui.ShowCompactMessageDialog(fm.window, "Trash", fmt.Sprintf("Queued %d item(s) to Trash.", len(srcPaths)))
		}
		fm.FocusFileList()
	})
}
