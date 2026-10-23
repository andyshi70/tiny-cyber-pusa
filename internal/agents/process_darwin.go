//go:build darwin

package agents

import (
	"os/exec"
	"strings"
)

// processRunningAny reports whether any of the named applications is running.
// `ps -Aoc comm=` prints the full executable path of every process; matching
// on the last path component keeps "ZCode" from also matching a random
// "SomeZCodeHelper" bundle helper.
func processRunningAny(names []string) bool {
	out, err := exec.Command("ps", "-Aoc", "comm=").Output()
	if err != nil {
		return false
	}
	running := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		path := strings.TrimSpace(line)
		if i := strings.LastIndex(path, "/"); i >= 0 {
			path = path[i+1:]
		}
		path = strings.TrimSuffix(path, ".app")
		if path != "" {
			running[strings.ToLower(path)] = true
		}
	}
	for _, n := range names {
		if running[strings.ToLower(strings.TrimSuffix(n, ".exe"))] {
			return true
		}
	}
	return false
}
