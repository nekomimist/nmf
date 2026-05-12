package display

// Info describes the primary display available during startup.
type Info struct {
	Available   bool
	Name        string
	Width       int
	Height      int
	WorkWidth   int
	WorkHeight  int
	PixelWidth  int
	PixelHeight int
	Scale       float32
}
