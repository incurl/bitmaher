//go:build linux

package graphics

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
	t.Setenv(alwaysSoftware, "")
	t.Setenv("WEBKIT_RENDERER", "pci:vmwgfx")

	Configure()

	if os.Getenv(webkitDisableCompositing) != "1" {
		t.Error("the compositing should be turned off on a software renderer")
	}
	// GTK has a renderer of its own, and it is the one that creates the
	// GL context the driver complains about.
	if os.Getenv(gtkSoftwareRenderer) != gtkCairoRenderer {
		t.Error("GTK should draw on the CPU on a software renderer")
	}
	// The graphics stack is loaded even with the two settings above, and
	// it still opens the render node of the card.
	if os.Getenv(alwaysSoftware) != "1" {
		t.Error("the graphics stack should stay on the CPU on a software renderer")
	}
}

// TestTheGraphicsAreKeptOnARealGPU verifies that a machine with a GPU
// keeps it: the interface is drawn by it, and it is smoother.
func TestTheGraphicsAreKeptOnARealGPU(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv(alwaysSoftware, "")
	t.Setenv("WEBKIT_RENDERER", "NVIDIA GeForce RTX 3080/PCIe/SSE2")

	Configure()

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
// The environment variables are the way to turn the rendering on and
// off by hand, and a user who has set one has said what they want.
func TestTheUserKeepsTheirOwnSetting(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "0")
	t.Setenv(gtkSoftwareRenderer, "gl")
	t.Setenv("WEBKIT_RENDERER", "pci:vmwgfx")

	Configure()

	if os.Getenv(webkitDisableCompositing) != "0" {
		t.Error("the setting of the user should be kept")
	}
	if os.Getenv(gtkSoftwareRenderer) != "gl" {
		t.Error("the setting of the user should be kept")
	}
}

// TestTheTwoSettingsAreIndependent verifies that one setting of the
// user does not silence the other.
//
// They used to be one: the setting of the compositing of WebKit stopped
// the renderer of GTK from being set, and the driver went on being
// asked to destroy the surface.
func TestTheTwoSettingsAreIndependent(t *testing.T) {
	t.Setenv(webkitDisableCompositing, "1")
	t.Setenv(gtkSoftwareRenderer, "")
	t.Setenv(alwaysSoftware, "")
	t.Setenv("WEBKIT_RENDERER", "pci:vmwgfx")

	Configure()

	if os.Getenv(webkitDisableCompositing) != "1" {
		t.Error("the setting of the user should be kept")
	}
	if os.Getenv(gtkSoftwareRenderer) != gtkCairoRenderer {
		t.Error("the renderer of GTK should be set even when the other one is")
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
	t.Setenv(alwaysSoftware, "")
	t.Setenv("WEBKIT_RENDERER", "")

	// The machine of the check decides what is expected.
	virtual := isVirtualMachineVendor(machineVendor(t))

	if virtual {
		// The machine is a virtual one: its renderer is a software
		// one, and the graphics are turned off.
		Configure()
		if os.Getenv(webkitDisableCompositing) != "1" {
			t.Error("the compositing should be turned off on a virtual machine")
		}
		return
	}

	if !hasHardwareRenderer() {
		t.Error("an unknown renderer should be left alone")
	}

	Configure()

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
