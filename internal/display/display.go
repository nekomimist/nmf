package display

// Info describes the primary display available during startup.
type Info struct {
	Available    bool
	Name         string
	Width        int
	Height       int
	WorkWidth    int
	WorkHeight   int
	PixelWidth   int
	PixelHeight  int
	Scale        float32
	DisplayScale float32
	UserScale    float32
}

const (
	baselineDPI = 120.0
	scaleAuto   = float32(-1.0)
	scaleEnvKey = "FYNE_SCALE"
)
