// Package notify sends desktop notifications. Notifications are fired only
// for events the user can't already see in a focused window.
package notify

import (
	"strings"
	"sync"
)

var (
	mu       sync.Mutex
	disabled bool
)

// SetEnabled toggles the whole channel (settings switch).
func SetEnabled(on bool) {
	mu.Lock()
	disabled = !on
	mu.Unlock()
}

// Enabled reports current state.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()
	return !disabled
}

// want trims the payload and reports whether a notification should be shown.
func want(title, body string) (string, string, bool) {
	if !Enabled() || strings.TrimSpace(title) == "" {
		return "", "", false
	}
	return title, body, true
}
