//go:build linux

package main

import (
	"os"
	"testing"
)

// TestTheSoftwareRenderersAreRecognised verifies that the renderers
// that tear the surface down on the wrong thread are known.
//
// The VMware renderer of a virtual machine is the one that prints
// "context mismatch in svga_surface_destroy", and Mesa's own software
// renderers have the same habit.
func TestTheSoftwareRenderersAreRecognised(t *testing.T) {
	software := []string{
		"llvmpipe (LLVM 15.0.7, 256 bits)",
		"softpipe",
		"swrast",
		"Software Rasterizer",
		"SVGA3D (VMware SVGA3D adapter)",
		"vmwgfx",
		"pci:vmwgfx",
	}

	for _, renderer := range software {
		if !isSoftwareRenderer(renderer) {
			t.Errorf("the renderer %q should be a software one", renderer)
		}
	}

	hardware := []string{
		"NVIDIA GeForce RTX 3080/PCIe/SSE2",
		"AMD Radeon RX 6800",
		"Intel(R) UHD Graphics 620",
		"Apple M1 Pro",
		"Mesa Intel(R) UHD Graphics 620 (KBL GT2)",
	}

	for _, renderer := range hardware {
		if isSoftwareRenderer(renderer) {
			t.Errorf("the renderer %q should not be a software one", renderer)
		}
	}
}

// TestTheGraphicsAreTurnedOffOnASoftwareRenderer verifies that the
// compositing is turned off when the machine has no GPU to draw with.
func TestTheGraphicsAreTurnedOffOnASoftwareRenderer(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv("WEBKIT_RENDERER", "pci:vmwgfx")

	configureGraphics()

	if os.Getenv(webkitDisableCompositing) != "1" {
		t.Error("the compositing should be turned off on a software renderer")
	}
	// GTK has a renderer of its own, and it is the one that creates the
	// GL context the driver complains about.
	if os.Getenv(gtkSoftwareRenderer) != gtkCairoRenderer {
		t.Error("GTK should draw on the CPU on a software renderer")
	}
}

// TestTheGraphicsAreKeptOnARealGPU verifies that a machine with a GPU
// keeps it: the interface is drawn by it, and it is smoother.
func TestTheGraphicsAreKeptOnARealGPU(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv("WEBKIT_RENDERER", "NVIDIA GeForce RTX 3080/PCIe/SSE2")

	configureGraphics()

	if os.Getenv(webkitDisableCompositing) != "" {
		t.Error("the compositing should be left alone on a machine with a GPU")
	}
	if os.Getenv(gtkSoftwareRenderer) != "" {
		t.Error("GTK should keep its renderer on a machine with a GPU")
	}
}

// TestTheUserKeepsTheirOwnSetting verifies that a setting of the user
// is not touched.
//
// The environment variable is the way to turn the compositing on and
// off by hand, and a user who has set it has said what they want.
func TestTheUserKeepsTheirOwnSetting(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "0")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv("WEBKIT_RENDERER", "pci:vmwgfx")

	configureGraphics()

	if os.Getenv(webkitDisableCompositing) != "0" {
		t.Error("the setting of the user should be kept")
	}
}

// TestAnUnknownRendererIsLeftAlone verifies that a machine whose
// renderer is not known keeps its graphics.
//
// The cost of a wrong guess is the smoothness of the interface, and
// the message it avoids is only a message.
//
// The machine the check runs on is a part of the answer: a virtual
// machine is known to have no real card, so the graphics are turned
// off there whatever the environment says.
func TestAnUnknownRendererIsLeftAlone(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv("WEBKIT_RENDERER", "")

	// The machine of the check decides what is expected.
	virtual := isVirtualMachineVendor(machineVendor(t))

	if virtual {
		// The machine is a virtual one: its renderer is a software
		// one, and the graphics are turned off.
		configureGraphics()
		if os.Getenv(webkitDisableCompositing) != "1" {
			t.Error("the compositing should be turned off on a virtual machine")
		}
		return
	}

	if !hasHardwareRenderer() {
		t.Error("an unknown renderer should be left alone")
	}

	configureGraphics()

	if os.Getenv(webkitDisableCompositing) != "" {
		t.Error("an unknown renderer should keep the compositing")
	}
}

// machineVendor reads the maker of the machine the check runs on.
func machineVendor(t *testing.T) string {
	t.Helper()

	vendor, err := os.ReadFile("/sys/class/dmi/id/sys_vendor")
	if err != nil {
		return ""
	}
	return string(vendor)
}

// TestTheVirtualMachinesAreRecognised verifies that the machines that
// have no real card are known.
//
// The renderer of a virtual machine is a software one, and it is the
// one that tears the surface down on the wrong thread.
func TestTheVirtualMachinesAreRecognised(t *testing.T) {
	virtual := []string{
		"VMware, Inc.",
		"innotek GmbH",
		"Oracle Corporation",
		"QEMU",
		"Parallels",
	}

	for _, vendor := range virtual {
		if !isVirtualMachineVendor(vendor) {
			t.Errorf("the vendor %q should be a virtual machine", vendor)
		}
	}

	real := []string{
		"LENOVO",
		"Dell Inc.",
		"Hewlett-Packard",
		"Apple Inc.",
	}

	for _, vendor := range real {
		if isVirtualMachineVendor(vendor) {
			t.Errorf("the vendor %q should not be a virtual machine", vendor)
		}
	}
}
