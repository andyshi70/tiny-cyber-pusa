//go:build darwin

package agents

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"zen-gate/internal/store"
)

// Autostart on macOS is a per-user launchd job: a plist in
// ~/Library/LaunchAgents with RunAtLoad. That is the same scope as the
// HKCU\...\Run entry used on Windows — it starts the app when the user logs
// in, without asking for admin rights.

// launchdLabel is the reverse-DNS job label; it doubles as the plist name.
const launchdLabel = "com.lagcomcom.zen-gate"

func launchdPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

// launchdDomain is the per-user launchd domain (`launchctl bootstrap`'s
// domain-target) and launchdServiceTarget the same job addressed by label
// (`launchctl print`/`bootout` take a service-target instead).
func launchdDomain() string        { return "gui/" + strconv.Itoa(os.Getuid()) }
func launchdServiceTarget() string { return launchdDomain() + "/" + launchdLabel }

// AutostartEnabled reports whether the launchd job is installed.
func AutostartEnabled() bool {
	p := launchdPlistPath()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// AutostartSet installs or removes the launchd job.
func AutostartSet(enable bool) error {
	plist := launchdPlistPath()
	if plist == "" {
		return fmt.Errorf("cannot resolve the user's home directory")
	}
	if !enable {
		// bootout is a no-op when the job is not loaded; ignore the error so a
		// stale plist can still be cleaned up.
		_ = exec.Command("launchctl", "bootout", launchdServiceTarget()).Run()
		if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
</dict>
</plist>
`, launchdLabel, plistXMLEscape(exe), logPath("zen-gate-launchd.out"), logPath("zen-gate-launchd.err"))
	tmp := plist + ".tmp"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, plist); err != nil {
		return err
	}

	// Re-bootstrap so a toggle takes effect without the next login. bootout
	// first: bootstrap refuses a domain that already holds the label.
	_ = exec.Command("launchctl", "bootout", launchdServiceTarget()).Run()
	if out, err := exec.Command("launchctl", "bootstrap", launchdDomain(), plist).CombinedOutput(); err != nil {
		// Older macOS (and any launchd hiccup) still has the plist on disk,
		// which is what AutostartEnabled reports — surface the log anyway.
		return fmt.Errorf("launchctl bootstrap: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func plistXMLEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// logPath points launchd's stdout/stderr at the zen-gate data dir so a
// launch-at-login instance is debuggable without a terminal.
func logPath(name string) string { return filepath.Join(store.Home(), name) }
