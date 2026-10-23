// zgwindow.m implements the macOS window: a frameless NSWindow (full-size
// content view, hidden traffic lights) with a WKWebView filling it, driving
// the same HTML dashboard the Windows build renders in WebView2.
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>

#include "zgwindow.h"

// Implemented in Go (see the //export in window_darwin.go): decides whether
// closing hides to the tray or quits.
extern void zgGoClose(void);

// Implemented in Go: builds the window. Called from zg_dispatch_create.
extern void zgGoCreate(void);

static NSWindow *gWindow = nil;
static WKWebView *gWebView = nil;
static id gDelegate = nil;
static BOOL gMaximized = NO;

// Scratch for the synchronous getters. Never handed to Go as a pointer of Go
// memory — Go copies out of them after the blocking call returns.
static int gBounds[4] = {0, 0, 0, 0};
static int gScreen[4] = {0, 0, 0, 0};

// Injected at document start so the dashboard's title bar finds the same
// window.zengate* globals the Windows build exposes via WebView2's Bind.
static NSString *const kShimJS = @""
  "(function(){"
  "  if (window.zengateShimInstalled) return;"
  "  window.zengateShimInstalled = true;"
  "  function send(n){ try { window.webkit.messageHandlers.zengateMsg.postMessage(n); } catch (e) {} }"
  "  window.zengateDrag = function(){ send('drag'); };"
  "  window.zengateMinimize = function(){ send('min'); };"
  "  window.zengateToggleMax = function(){ send('max'); };"
  "  window.zengateClose = function(){ send('close'); };"
  "})();";

@interface ZenGateDelegate : NSObject <NSWindowDelegate, WKScriptMessageHandler>
@end

@implementation ZenGateDelegate

// Both the dashboard's × and ⌘W land here; Go owns the close decision, so the
// window never actually closes on its own.
- (BOOL)windowShouldClose:(NSWindow *)sender {
  zgGoClose();
  return NO;
}

// Zooming is the macOS "maximize"; AppKit remembers the pre-zoom frame so
// zoom:nil toggles back. Track the state from the frame rather than at the
// call site so user resizes are reflected too.
- (void)windowDidResize:(NSNotification *)note {
  if (!gWindow) {
    return;
  }
  NSRect f = gWindow.frame;
  NSRect v = gWindow.screen.visibleFrame;
  gMaximized = (f.size.width >= v.size.width - 1 && f.size.height >= v.size.height - 1);
}

- (void)userContentController:(WKUserContentController *)controller
       didReceiveScriptMessage:(WKScriptMessage *)message {
  NSString *name = [message.body isKindOfClass:[NSString class]] ? message.body : @"";
  if ([name isEqualToString:@"drag"]) {
    // performWindowDragWithEvent hands the gesture to AppKit's move loop, and
    // needs the live event — which is what the page is calling from.
    NSEvent *event = [NSApp currentEvent];
    if (event && gWindow) {
      [gWindow performWindowDragWithEvent:event];
    }
  } else if ([name isEqualToString:@"min"]) {
    [gWindow miniaturize:nil];
  } else if ([name isEqualToString:@"max"]) {
    [gWindow zoom:nil];
  } else if ([name isEqualToString:@"close"]) {
    zgGoClose();
  }
}

@end

// --- main-thread dispatch ----------------------------------------------------
//
// AppKit is main-thread-only and the Go side calls in from any goroutine (the
// tray menu handlers, the update loop, admin requests), so every operation is
// bounced onto the main queue. Running inline when we are already there keeps
// the calls synchronous for the operations that are read back immediately.

static void zgOnMain(void (^block)(void)) {
  if ([NSThread isMainThread]) {
    block();
    return;
  }
  dispatch_async(dispatch_get_main_queue(), block);
}

static void zgOnMainSync(void (^block)(void)) {
  if ([NSThread isMainThread]) {
    block();
    return;
  }
  dispatch_sync(dispatch_get_main_queue(), block);
}

void zg_dispatch_create(void) {
  if ([NSThread isMainThread]) {
    zgGoCreate();
    return;
  }
  dispatch_async(dispatch_get_main_queue(), ^{
    zgGoCreate();
  });
}

void zg_run(const char *title, int width, int height, const char *url) {
  if (gWindow != nil) {
    return; // already built — systray's ready hook fires exactly once
  }

  NSString *titleStr = [[NSString alloc] initWithUTF8String:title];
  NSUInteger mask = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                    NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable |
                    NSWindowStyleMaskFullSizeContentView;
  gWindow = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, width, height)
                                          styleMask:mask
                                            backing:NSBackingStoreBuffered
                                              defer:NO];
  // The dashboard draws its own title bar, so the native one must not paint —
  // and the traffic lights go with it, since the HTML has its own buttons.
  gWindow.title = titleStr;
  gWindow.titlebarAppearsTransparent = YES;
  gWindow.titleVisibility = NSWindowTitleHidden;
  gWindow.releasedWhenClosed = NO;
  gWindow.minSize = NSMakeSize(600, 420);
  gWindow.backgroundColor = [NSColor colorWithCalibratedRed:0.055
                                                      green:0.055
                                                       blue:0.055
                                                      alpha:1.0];
  for (NSNumber *button in @[ @(NSWindowCloseButton), @(NSWindowMiniaturizeButton), @(NSWindowZoomButton) ]) {
    [gWindow standardWindowButton:(NSWindowButton)[button unsignedIntValue]].hidden = YES;
  }

  gDelegate = [[ZenGateDelegate alloc] init];
  gWindow.delegate = (id<NSWindowDelegate>)gDelegate;
  [gWindow center];

  WKUserContentController *content = [[WKUserContentController alloc] init];
  [content addScriptMessageHandler:(id<WKScriptMessageHandler>)gDelegate name:@"zengateMsg"];
  [content addUserScript:[[WKUserScript alloc]
                             initWithSource:kShimJS
                              injectionTime:WKUserScriptInjectionTimeAtDocumentStart
                           forMainFrameOnly:YES]];

  WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
  config.userContentController = content;
  gWebView = [[WKWebView alloc] initWithFrame:gWindow.contentView.bounds
                                 configuration:config];
  gWebView.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
  // Let the page's own background show through instead of white.
  [gWebView setValue:@(NO) forKey:@"drawsBackground"];
  [gWindow.contentView addSubview:gWebView];

  NSURL *nsURL = [NSURL URLWithString:[[NSString alloc] initWithUTF8String:url]];
  if (nsURL) {
    [gWebView loadRequest:[NSURLRequest requestWithURL:nsURL]];
  }
}

void zg_do_minimize(void) { zgOnMain(^{ [gWindow miniaturize:nil]; }); }

void zg_do_toggle_max(void) { zgOnMain(^{ [gWindow zoom:nil]; }); }

void zg_do_show(void) {
  zgOnMain(^{
    if (!gWindow) {
      return;
    }
    [gWindow makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
  });
}

void zg_do_hide(void) { zgOnMain(^{ [gWindow orderOut:nil]; }); }

void zg_do_terminate(void) { zgOnMain(^{ [NSApp terminate:nil]; }); }

int zg_do_is_visible(void) {
  __block int result = 0;
  zgOnMainSync(^{ result = (gWindow != nil && gWindow.isVisible) ? 1 : 0; });
  return result;
}

int zg_do_is_minimized(void) {
  __block int result = 0;
  zgOnMainSync(^{ result = (gWindow != nil && gWindow.isMiniaturized) ? 1 : 0; });
  return result;
}

int zg_do_is_maximized(void) {
  __block int result = 0;
  zgOnMainSync(^{ result = gMaximized ? 1 : 0; });
  return result;
}

void zg_do_set_bounds(int x, int y, int w, int h) {
  zgOnMain(^{
    if (!gWindow) {
      return;
    }
    // Find the display that owns the requested top-left corner.
    NSScreen *target = [NSScreen mainScreen];
    for (NSScreen *s in [NSScreen screens]) {
      NSRect sf = s.frame;
      if (x >= (int)lround(sf.origin.x) && x < (int)lround(sf.origin.x + sf.size.width) &&
          y >= 0 && y < (int)lround(sf.size.height)) {
        target = s;
        break;
      }
    }
    NSRect sf = target.frame;
    [gWindow setFrame:NSMakeRect(sf.origin.x + x, sf.origin.y + sf.size.height - y - h, w, h)
              display:YES];
  });
}

void zg_do_get_bounds(void) {
  zgOnMainSync(^{
    if (!gWindow) {
      gBounds[0] = gBounds[1] = gBounds[2] = gBounds[3] = 0;
      return;
    }
    NSRect f = gWindow.frame;
    NSScreen *screen = gWindow.screen ?: [NSScreen mainScreen];
    NSRect sf = screen.frame;
    // Flip the y axis and rebase onto the screen the window sits on.
    gBounds[0] = (int)lround(f.origin.x - sf.origin.x);
    gBounds[1] = (int)lround(sf.size.height - (f.origin.y + f.size.height));
    gBounds[2] = (int)lround(f.size.width);
    gBounds[3] = (int)lround(f.size.height);
  });
}

void zg_do_get_screen_frame(void) {
  zgOnMainSync(^{
    NSRect sf = [NSScreen mainScreen].frame;
    gScreen[0] = (int)lround(sf.origin.x);
    gScreen[1] = (int)lround(sf.origin.y);
    gScreen[2] = (int)lround(sf.size.width);
    gScreen[3] = (int)lround(sf.size.height);
  });
}

int *zg_bounds(void) { return gBounds; }

int *zg_screen(void) { return gScreen; }

void zg_noop_string(const char *unused) {}

void zg_free(void *p) { free(p); }