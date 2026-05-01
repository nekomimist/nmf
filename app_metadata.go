package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

const (
	appID          = "io.github.nekomimist.nmf"
	appDisplayName = "NMF"
)

//go:embed nmf-icon.png
var appIconBytes []byte

var appIconResource = fyne.NewStaticResource("nmf-icon.png", appIconBytes)
