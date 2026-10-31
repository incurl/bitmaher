//go:build !linux

package graphics

// configureGraphics decides how the webview is drawn.
//
// The work is only needed on Linux, where the software renderers of a
// virtual machine tear the surface down on the wrong thread. The other
// platforms have nothing to do here.
func Configure() {}
