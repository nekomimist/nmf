package ui

import "fyne.io/fyne/v2"

const (
	treeDialogIconSize      float32 = 16
	treeDialogTreeWidth     float32 = 500
	treeDialogTreeHeight    float32 = 400
	treeDialogContentWidth  float32 = 550
	treeDialogContentHeight float32 = 500

	searchDialogListWidth      float32 = 600
	searchDialogListHeight     float32 = 400
	searchDialogContentWidth   float32 = 650
	searchDialogContentHeight  float32 = 500
	responsiveDialogWidthRatio         = 0.90

	filterDialogListHeight float32 = 350

	copyMoveTargetListWidth  float32 = 500
	copyMoveTargetListHeight float32 = 160
	copyMoveDestListHeight   float32 = 260

	compareDialogWidth        float32 = searchDialogListWidth
	compareDialogListHeight   float32 = 240
	compareSourcePathMaxRunes         = 72

	sortDialogWidth  float32 = 400
	sortDialogHeight float32 = 350

	quitDialogWidth  float32 = 460
	quitDialogHeight float32 = 64
	quitDialogGap    float32 = 18
	quitDialogBottom float32 = 14

	smbLoginDialogWidth  float32 = 420
	smbLoginDialogHeight float32 = 200

	archivePasswordDialogWidth  float32 = 420
	archivePasswordDialogHeight float32 = 140

	lineEditDialogWidth    float32 = 640
	lineEditDialogHeight   float32 = 160
	renameDialogMaxWidth   float32 = 960
	renameDialogWidthRatio         = 0.70

	conflictDialogWidth float32 = 620

	deleteDialogWidth      float32 = 560
	deleteTargetListHeight float32 = 170

	maintenanceDialogWidth  float32 = 760
	maintenanceDialogHeight float32 = 520
	maintenanceListHeight   float32 = 260

	fileViewerFallbackWidth  float32 = 900
	fileViewerFallbackHeight float32 = 760
	fileViewerWidthRatio     float32 = 0.96
	fileViewerHeightRatio    float32 = 0.88
	fileViewerSearchWidth    float32 = 260
	fileViewerLineWidth      float32 = 90

	jobsDetailsWidth  float32 = 680
	jobsDetailsHeight float32 = 140
	jobsWindowWidth   float32 = 720
	jobsWindowHeight  float32 = 480

	compactMessageWidth        float32 = 520
	compactMessageMinHeight    float32 = 72
	compactMessageLineHeight   float32 = 28
	compactMessageVPadding     float32 = 24
	compactDialogExtraWidth    float32 = 40
	compactDialogExtraHeight   float32 = 92
	compactMessageCharsPerLine         = 52

	versionDialogLabelWidth        float32 = 135
	versionDialogValueWidth        float32 = compactMessageWidth - versionDialogLabelWidth - 20
	versionDialogRowHeight         float32 = 34
	versionDialogWrappedLineHeight float32 = 28
	versionDialogValueCharsPerLine         = 34
)

func metricsSize(width, height float32) fyne.Size {
	return fyne.NewSize(width, height)
}

func parentCanvasWidth(parent fyne.Window) float32 {
	if parent == nil || parent.Canvas() == nil {
		return 0
	}
	return parent.Canvas().Size().Width
}

func responsiveDialogWidth(parent fyne.Window, minimum float32) float32 {
	return responsiveDialogWidthWithRatio(parent, minimum, responsiveDialogWidthRatio, 0)
}

func responsiveDialogWidthWithRatio(parent fyne.Window, minimum, ratio, maximum float32) float32 {
	width := minimum
	if parentWidth := parentCanvasWidth(parent); parentWidth > 0 && ratio > 0 {
		width = fyne.Max(width, parentWidth*ratio)
	}
	if maximum > 0 && width > maximum {
		width = maximum
	}
	return width
}

func RenameDialogWidthRatio() float32 {
	return renameDialogWidthRatio
}

func RenameDialogMaxWidth() float32 {
	return renameDialogMaxWidth
}

func searchDialogListSize() fyne.Size {
	return metricsSize(searchDialogListWidth, searchDialogListHeight)
}

func searchDialogContentSize() fyne.Size {
	return metricsSize(searchDialogContentWidth, searchDialogContentHeight)
}

func filterDialogListSize() fyne.Size {
	return metricsSize(searchDialogListWidth, filterDialogListHeight)
}
