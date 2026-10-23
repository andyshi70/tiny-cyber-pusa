//go:build darwin

// macOS notifications, delivered through osascript's `display notification`.
// The title/body are passed as argv rather than interpolated into the script,
// so quotes and newlines in model names cannot break the AppleScript.
package notify

import "os/exec"

var osascriptScript = []string{
	"-e", `on run argv`,
	`-e`, `display notification (item 2 of argv) with title (item 1 of argv)`,
	"-e", `end run`,
}

// Toast shows a notification titled "Zen Gate".
// Best-effort: osascript delivery failures are silently ignored, as is the
// case when the user has denied notifications to the app in System Settings.
func Toast(title, body string) {
	title, body, ok := want(title, body)
	if !ok {
		return
	}
	args := append(append([]string{}, osascriptScript...), title, body)
	_ = exec.Command("osascript", args...).Start()
}
