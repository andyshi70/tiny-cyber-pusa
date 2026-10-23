// Package window hosts the desktop UI: a native window embedding the
// platform's web view (WebView2 on Windows, WKWebView on macOS), rendering the
// local gateway's dashboard.
//
// The package also owns the UI thread. Windows is happy with the tray loop on
// its own locked goroutine while the window owns the main thread; macOS
// requires both AppKit objects to be built on the main thread inside
// NSApp.run. Run therefore takes the tray configuration and does the platform
// dance itself, so callers stay identical on both.
package window

import (
	"fmt"
)

// Bounds is a persisted window rectangle, in screen coordinates with the
// origin at the top-left of the display the window was last on. That is the
// platform-neutral convention: macOS's own bottom-left origin is converted at
// the ObjC boundary so a saved rectangle round-trips unchanged.
type Bounds struct {
	X, Y, W, H int
	Maximized  bool
}

// Options configures the main window.
type Options struct {
	Title string
	URL   string
	// DataPath is the web view's user-data directory. Windows-only: WebView2
	// needs an explicit folder, WKWebView stores its own inside the app
	// container.
	DataPath string
	Width    int
	Height   int
	// Bounds, when set with W>0, restores a persisted rectangle/maximized state.
	Bounds *Bounds
	// Tray configures the menu-bar / notification-area icon. Nil disables it,
	// leaving the app as a window with no way back to it once closed.
	Tray *TrayOptions
	// OnReady fires once the native window exists (hwnd available).
	OnReady func(hwnd uintptr)
	// OnCloseButton is invoked by the HTML title bar's ×. Returning false
	// keeps the window open (close-to-tray); true quits the app.
	OnCloseButton func() bool
}

// TrayOptions configures the notification-area icon. It mirrors tray.Options
// so callers pass one value in and the window layer decides how to host it on
// the platform's UI thread.
type TrayOptions struct {
	DashboardURL string
	OnQuit       func()
	OnReprobe    func()
	OnShow       func()
}

var diag func(string)

// SetDiag wires a log sink for window/icon diagnostics.
func SetDiag(f func(string)) { diag = f }

func dlogf(format string, args ...any) {
	if diag != nil {
		diag(fmt.Sprintf(format, args...))
	}
}
