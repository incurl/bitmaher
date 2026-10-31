//go:build linux

package main

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

// configureGraphics decides how the webview is drawn.
//
// It is called before the application is created, because the engine
// reads the environment when the webview is made.
func configureGraphics() {
	// The user has spoken: their setting is kept.
	if os.Getenv(webkitDisableCompositing) != "" {
		return
	}

	// The software renderers are the ones that tear the surface down
	// on the wrong thread. A machine with a real GPU keeps it.
	if hasHardwareRenderer() {
		return
	}

	_ = os.Setenv(webkitDisableCompositing, "1")

	// GTK draws the window with a renderer of its own, and that one
	// creates the GL context. It is told to draw on the CPU as well.
	if os.Getenv(gtkSoftwareRenderer) == "" {
		_ = os.Setenv(gtkSoftwareRenderer, gtkCairoRenderer)
	}
}
