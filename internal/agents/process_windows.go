//go:build windows

package agents

import (
	"os/exec"
	"strings"
	"syscall"
)

// processRunningAny reports whether any of the named applications is running.
// Windows is matched by image name, so "ZCode" is looked up as "ZCode.exe".
func processRunningAny(names []string) bool {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	// A GUI process spawning tasklist would otherwise flash a black console
	// window on every dashboard poll.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(out))
	for _, n := range names {
		for _, candidate := range []string{n, n + ".exe"} {
			if strings.Contains(lower, strings.ToLower(candidate)) {
				return true
			}
		}
	}
	return false
}
