//go:build linux

package graphics

import "os"

// The graphics of the webview on Linux.
//
// The webview is rendered by WebKitGTK, which draws through the GPU.
// On a machine whose only renderer is the software one of a virtual
// machine, that driver tears the surface down on a thread of its own
// when the window goes away, and it says so:
//
//	context mismatch in svga_surface_destroy
//
// The message comes from the driver, not from this application: it is
// the VMware SVGA3D Gallium driver that prints it, when it destroys a
// surface that belongs to another context. It is printed on the way
// out, and it is harmless.
//
// It is still noise that the user does not need to see, and on some
// machines the same race leaves a window that does not repaint. The
// compositing is therefore turned off, so that the webview draws on
// the CPU and the driver is never asked to destroy a surface.
//
// The setting is only applied when the user has not said otherwise:
// an environment variable that is already set wins, so that a user
// with a working GPU setup keeps it.

// The environment variable that turns the GPU compositing of WebKitGTK
// off.
const webkitDisableCompositing = "WEBKIT_DISABLE_COMPOSITING_MODE"

// The environment variable that tells GTK to draw on the CPU.
//
// GTK has a renderer of its own, and it is the one that creates the GL
// context the driver complains about. The WebKit setting above is not
// enough: the context is created by GTK, and the driver is asked to
// destroy the surface either way.
const gtkSoftwareRenderer = "GSK_RENDERER"

// The value of that variable that draws on the CPU.
const gtkCairoRenderer = "cairo"

// The environment variable that keeps the graphics stack on the CPU.
//
// The two settings above are not enough. They stop WebKit and GTK from
// drawing with the GPU, but the graphics stack is still loaded, and it
// still opens the render node of the card: the VMware driver, which is
// the one that tears the surface down on the wrong thread, is asked to
// destroy a surface it never made. This one keeps the render node
// closed, so the driver is never engaged at all.
const alwaysSoftware = "LIBGL_ALWAYS_SOFTWARE"

// Configure decides how the webview is drawn.
//
// It is called before the application is created, because the engine
// reads the environment when the webview is made.
func Configure() {
	// The two settings are separate, and a user may set either of them.
	// They used to be one: a setting of the first one stopped the
	// second one from being applied, and the driver went on being
	// asked to destroy the surface.
	//
	// A machine with a real GPU keeps its rendering, and the setting
	// of the user wins over both.
	if hasHardwareRenderer() {
		return
	}

	if os.Getenv(webkitDisableCompositing) == "" {
		_ = os.Setenv(webkitDisableCompositing, "1")
	}

	// GTK draws the window with a renderer of its own, and that one
	// creates the GL context. It is told to draw on the CPU as well.
	if os.Getenv(gtkSoftwareRenderer) == "" {
		_ = os.Setenv(gtkSoftwareRenderer, gtkCairoRenderer)
	}

	// The graphics stack is still loaded with the two settings above,
	// and it still opens the render node of the card. This one keeps
	// the card closed, so that the driver is never asked to destroy a
	// surface it did not make.
	if os.Getenv(alwaysSoftware) == "" {
		_ = os.Setenv(alwaysSoftware, "1")
	}
}
