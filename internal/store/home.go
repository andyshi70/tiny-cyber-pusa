package store

import (
	"os"
	"path/filepath"
	"runtime"
)

// AppDataDir returns the per-user application data directory of the host OS:
//
//	Windows %APPDATA%\zen-gate
//	macOS   ~/Library/Application Support/zen-gate
//	other   $XDG_CONFIG_HOME (or ~/.config)/zen-gate
//
// ZEN_GATE_HOME overrides everything.
func AppDataDir() string {
	if h := os.Getenv("ZEN_GATE_HOME"); h != "" {
		return h
	}
	switch runtime.GOOS {
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "zen-gate")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Roaming", "zen-gate")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(os.TempDir(), "zen-gate")
		}
		return filepath.Join(home, "Library", "Application Support", "zen-gate")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "zen-gate")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "zen-gate")
	}
}
