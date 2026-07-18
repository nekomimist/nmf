package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"nmf/internal/config"
)

func TestFileListRowKeepsFixedRendererObjectsAcrossUpdates(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	row := NewFileListRow(
		config.CursorStyleConfig{Type: "underline", Thickness: 2},
		color.RGBA{R: 20, G: 30, B: 40, A: 255},
	)
	renderer := test.WidgetRenderer(row).(*fileListRowRenderer)
	before := append([]fyne.CanvasObject(nil), renderer.Objects()...)

	status := color.RGBA{R: 1, G: 2, B: 3, A: 80}
	row.SetDecorations(
		&status,
		true,
		color.RGBA{R: 4, G: 5, B: 6, A: 100},
		true,
		color.RGBA{R: 7, G: 8, B: 9, A: 255},
	)
	renderer.Refresh()

	after := renderer.Objects()
	if len(after) != len(before) {
		t.Fatalf("renderer objects = %d, want %d", len(after), len(before))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("renderer object %d changed identity: before=%T after=%T", i, before[i], after[i])
		}
	}
	if got := rgba(renderer.status.FillColor); got != status {
		t.Fatalf("status color = %#v, want %#v", got, status)
	}
	if got, want := rgba(renderer.selection.FillColor), (color.RGBA{R: 4, G: 5, B: 6, A: 100}); got != want {
		t.Fatalf("selection color = %#v, want %#v", got, want)
	}
	if got, want := rgba(renderer.cursorBottom.FillColor), (color.RGBA{R: 7, G: 8, B: 9, A: 255}); got != want {
		t.Fatalf("underline color = %#v, want %#v", got, want)
	}
	if got := rgba(renderer.cursorTop.FillColor); got.A != 0 {
		t.Fatalf("non-underline cursor color = %#v, want transparent", got)
	}

	row.SetDecorations(nil, false, color.RGBA{}, false, color.RGBA{})
	renderer.Refresh()
	for name, objectColor := range map[string]color.Color{
		"status":           renderer.status.FillColor,
		"selection":        renderer.selection.FillColor,
		"cursorBackground": renderer.cursorBackground.FillColor,
		"cursorTop":        renderer.cursorTop.FillColor,
		"cursorBottom":     renderer.cursorBottom.FillColor,
		"cursorLeft":       renderer.cursorLeft.FillColor,
		"cursorRight":      renderer.cursorRight.FillColor,
	} {
		if got := rgba(objectColor); got.A != 0 {
			t.Fatalf("%s color = %#v, want transparent", name, got)
		}
	}
}

func TestFileListRowRendererLayoutTracksResize(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	row := NewFileListRow(
		config.CursorStyleConfig{Type: "border", Thickness: 3},
		color.RGBA{A: 255},
	)
	renderer := test.WidgetRenderer(row).(*fileListRowRenderer)

	for _, size := range []fyne.Size{
		fyne.NewSize(240, 24),
		fyne.NewSize(420, 36),
	} {
		row.Resize(size)

		for name, object := range map[string]fyne.CanvasObject{
			"content":          row.content,
			"status":           renderer.status,
			"selection":        renderer.selection,
			"cursorBackground": renderer.cursorBackground,
		} {
			if got := object.Size(); got != size {
				t.Fatalf("%s size after row resize to %v = %v", name, size, got)
			}
		}
		if got, want := renderer.cursorBottom.Position(), fyne.NewPos(0, size.Height-3); got != want {
			t.Fatalf("bottom cursor position after row resize to %v = %v, want %v", size, got, want)
		}
		if got, want := renderer.cursorRight.Position(), fyne.NewPos(size.Width-3, 0); got != want {
			t.Fatalf("right cursor position after row resize to %v = %v, want %v", size, got, want)
		}
		if got, want := renderer.cursorBottom.Size(), fyne.NewSize(size.Width, 3); got != want {
			t.Fatalf("bottom cursor size after row resize to %v = %v, want %v", size, got, want)
		}
		if got, want := renderer.cursorRight.Size(), fyne.NewSize(3, size.Height); got != want {
			t.Fatalf("right cursor size after row resize to %v = %v, want %v", size, got, want)
		}
	}
}

func TestFileListRowCursorStyles(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	cursorColor := color.RGBA{R: 100, G: 110, B: 120, A: 200}
	tests := []struct {
		name             string
		style            string
		wantBackground   color.RGBA
		wantTop          color.RGBA
		wantBottom       color.RGBA
		wantLeftAndRight color.RGBA
	}{
		{
			name:       "underline",
			style:      "underline",
			wantBottom: cursorColor,
		},
		{
			name:             "border",
			style:            "border",
			wantTop:          cursorColor,
			wantBottom:       cursorColor,
			wantLeftAndRight: cursorColor,
		},
		{
			name:           "background",
			style:          "background",
			wantBackground: scaleAlpha(cursorColor, backgroundCursorAlphaScale),
		},
		{
			name:       "unsupported style falls back to underline",
			style:      "icon",
			wantBottom: cursorColor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := NewFileListRow(config.CursorStyleConfig{Type: tt.style, Thickness: 2}, color.RGBA{A: 255})
			renderer := test.WidgetRenderer(row).(*fileListRowRenderer)
			row.SetDecorations(nil, false, color.RGBA{}, true, cursorColor)
			renderer.Refresh()

			if got := rgba(renderer.cursorBackground.FillColor); got != tt.wantBackground {
				t.Fatalf("background cursor color = %#v, want %#v", got, tt.wantBackground)
			}
			if got := rgba(renderer.cursorTop.FillColor); got != tt.wantTop {
				t.Fatalf("top cursor color = %#v, want %#v", got, tt.wantTop)
			}
			if got := rgba(renderer.cursorBottom.FillColor); got != tt.wantBottom {
				t.Fatalf("bottom cursor color = %#v, want %#v", got, tt.wantBottom)
			}
			if got := rgba(renderer.cursorLeft.FillColor); got != tt.wantLeftAndRight {
				t.Fatalf("left cursor color = %#v, want %#v", got, tt.wantLeftAndRight)
			}
			if got := rgba(renderer.cursorRight.FillColor); got != tt.wantLeftAndRight {
				t.Fatalf("right cursor color = %#v, want %#v", got, tt.wantLeftAndRight)
			}
		})
	}
}

func TestFileListRowDecorationZOrder(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	row := NewFileListRow(config.CursorStyleConfig{Type: "background"}, color.RGBA{A: 255})
	renderer := test.WidgetRenderer(row).(*fileListRowRenderer)
	objects := renderer.Objects()

	want := []fyne.CanvasObject{
		row.content,
		renderer.status,
		renderer.selection,
		renderer.cursorBackground,
		renderer.cursorTop,
		renderer.cursorBottom,
		renderer.cursorLeft,
		renderer.cursorRight,
	}
	if len(objects) != len(want) {
		t.Fatalf("renderer objects = %d, want %d", len(objects), len(want))
	}
	for i := range want {
		if objects[i] != want[i] {
			t.Fatalf("z-order object %d = %T, want %T", i, objects[i], want[i])
		}
	}
}

func rgba(c color.Color) color.RGBA {
	return color.RGBAModel.Convert(c).(color.RGBA)
}

func BenchmarkFileListRowDecorationUpdate(b *testing.B) {
	app := test.NewApp()
	b.Cleanup(app.Quit)

	row := NewFileListRow(
		config.CursorStyleConfig{Type: "underline", Thickness: 2},
		color.RGBA{A: 255},
	)
	window := app.NewWindow("benchmark")
	window.SetContent(row)
	row.Resize(fyne.NewSize(800, 24))
	cursorColor := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row.SetDecorations(nil, false, color.RGBA{}, i%2 == 0, cursorColor)
	}
}
