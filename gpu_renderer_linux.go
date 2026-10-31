//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// hasHardwareRenderer answers whether the machine has a GPU that can
// draw the webview.
//
// The renderer is looked for in three places, in the order that keeps
// the answer the most certain:
//
//  1. the environment, where a user or a session sets it;
//  2. the graphics card of the machine, which says what the driver is;
//  3. the DMI of the machine, which says what the machine is.
//
// The software renderers are the ones that tear the surface down on
// the wrong thread.
func hasHardwareRenderer() bool {
	// The environment has the last word: it is what a user sets to
	// override everything else.
	if renderer := strings.TrimSpace(os.Getenv("WEBKIT_RENDERER")); renderer != "" {
		return !isSoftwareRenderer(renderer)
	}

	// The graphics card of the machine.
	if renderer := pciRenderer(); renderer != "" {
		return !isSoftwareRenderer(renderer)
	}

	// The machine itself. A virtual machine has no real card, and its
	// driver is the one that tears the surface down.
	if vendor, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		if isVirtualMachineVendor(string(vendor)) {
			return false
		}
	}

	// Nothing is known about the renderer. The webview is left alone:
	// a wrong guess costs the user the smoothness of the interface, and
	// the error it avoids is only a message.
	return true
}

// pciRenderer reads the name of the graphics card of the machine.
//
// The card is found through the PCI bus, and its name is read from
// the driver that has bound to it.
func pciRenderer() string {
	devices, err := filepath.Glob("/sys/bus/pci/devices/*/class")
	if err != nil {
		return ""
	}

	for _, class := range devices {
		// The class of a graphics card is 0x03xxxx.
		content, err := os.ReadFile(class)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(string(content)), "0x03") {
			continue
		}

		device := filepath.Dir(class)

		// The driver that has bound to the card.
		driver, err := filepath.Glob(filepath.Join(device, "driver", "module", "drivers", "*"))
		if err != nil || len(driver) == 0 {
			continue
		}
		name := filepath.Base(driver[0])
		if name != "" {
			return name
		}
	}

	return ""
}

// isVirtualMachineVendor answers whether the machine is a virtual one.
//
// The renderer of a virtual machine is a software one, and it is the
// one that tears the surface down on the wrong thread.
func isVirtualMachineVendor(vendor string) bool {
	name := strings.ToLower(vendor)

	for _, marker := range []string{
		"vmware",
		"virtualbox",
		"innotek",
		"qemu",
		"kvm",
		"parallels",
		"bochs",
		"oracle",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}

	return false
}

// isSoftwareRenderer answers whether the renderer is a software one.
//
// The names are the ones the graphics stack uses: llvmpipe is Mesa's
// software renderer, swrast is the software rasteriser of Mesa, and
// vmwgfx is the driver of the VMware card.
func isSoftwareRenderer(renderer string) bool {
	name := strings.ToLower(renderer)

	for _, marker := range []string{
		"llvmpipe",
		"softpipe",
		"swrast",
		"software",
		"svga3d",
		"vmwgfx",
		"vmware",
	} {
		if strings.Contains(name, marker) {
			return true
		}
	}

	return false
}
