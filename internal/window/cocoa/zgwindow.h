// zgwindow.h is the whole C surface of the macOS window layer.
//
// Two shapes of call, both safe from any thread:
//   - zg_run builds the NSWindow + WKWebView. Main thread only, called once
//     from systray's ready hook inside NSApp.run.
//   - everything else is a zg_do_* operation that hops to the main queue
//     itself, because that is the only thread AppKit allows touching windows.
#ifndef ZGWINDOW_H
#define ZGWINDOW_H

extern void zg_run(const char *title, int width, int height, const char *url);

// Builds the window on the main queue. getlantern/systray hands its ready
// callback to a fresh goroutine, so the caller cannot rely on being on the main
// thread already; this bounces into NSApp.run's queue and calls back into Go's
// zgGoCreate().
extern void zg_dispatch_create(void);

// Frees a string allocated by Go's C.CString.
extern void zg_free(void *p);

// Stands in for the Windows-only SetAppUserModelID so cmd/zen-gate can stay
// platform-free.
extern void zg_noop_string(const char *unused);

// Window operations.
extern void zg_do_minimize(void);
extern void zg_do_toggle_max(void);
extern void zg_do_show(void);
extern void zg_do_hide(void);
extern void zg_do_terminate(void);
extern int zg_do_is_visible(void);
extern int zg_do_is_minimized(void);
extern int zg_do_is_maximized(void);

// Bounds use the platform-neutral top-left-origin convention documented on
// window.Bounds; the flip to AppKit's bottom-left screens happens in here.
extern void zg_do_set_bounds(int x, int y, int w, int h);
extern void zg_do_get_bounds(void);
extern void zg_do_get_screen_frame(void);

// Static C scratch buffers the two getters above fill. They are read straight
// after the (blocking) getter returns, so no Go pointer ever crosses into C.
extern int *zg_bounds(void);
extern int *zg_screen(void);

#endif