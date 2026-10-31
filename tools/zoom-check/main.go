// A live check of the zoom of the window.
//
// The size of the interface is the zoom of the webview. The check
// creates a window with a zoom of 125% and confirms that the window
// carries it, which is what the application does when the user changes
// the setting.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/incurl/bitmaher/internal/graphics"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// The zoom the check applies.
const zoom = 1.25

func main() {
	// The window of this check is drawn the way the application draws
	// it, so that the check does not leave the driver complaining on
	// the way out.
	graphics.Configure()

	app := application.New(application.Options{
		Name: "BitMaher",
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "BitMaher",
		Width:  800,
		Height: 600,
		URL:    "/",
		Zoom:   zoom,
	})

	go func() {
		// The window needs a moment before it can answer.
		time.Sleep(2 * time.Second)

		applied := window.GetZoom()
		fmt.Printf("1. The window was created with a zoom of %v\n", zoom)
		fmt.Printf("2. The zoom of the window is %v\n", applied)

		if applied < zoom-0.05 || applied > zoom+0.05 {
			fmt.Println("   FAIL: the zoom was not applied")
			app.Quit()
			os.Exit(1)
		}

		// The zoom can be changed after the fact, which is what the
		// application does when the user moves the slider.
		window.SetZoom(1.5)
		time.Sleep(500 * time.Millisecond)

		changed := window.GetZoom()
		fmt.Printf("3. The zoom was changed to %v\n", changed)
		if changed < 1.45 || changed > 1.55 {
			fmt.Println("   FAIL: the zoom was not changed")
			app.Quit()
			os.Exit(1)
		}

		fmt.Println("\nZOOM: PASS")
		app.Quit()
		os.Exit(0)
	}()

	app.Run()
}
