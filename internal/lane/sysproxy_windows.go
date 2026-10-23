//go:build windows

package lane

import (
	neturl "net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows/registry"
)

// System proxy support: proxy tools (v2rayN, Clash, …) publish themselves
// through the WinINET settings in HKCU when the user flips "系统代理" — NOT
// through environment variables, which is why the "env" mode never sees them.
// This reader picks those settings up, cached for a few seconds so toggling
// the proxy in the tool's UI is picked up without restarting zen-gate.

const sysProxyCacheTTL = 3 * time.Second

var (
	sysProxyMu      sync.Mutex
	sysProxyCached  *neturl.URL // nil = system proxy off
	sysProxyCacheAt time.Time
)

// SystemProxyURL returns the system proxy as a URL, or nil when disabled.
func SystemProxyURL() *neturl.URL {
	sysProxyMu.Lock()
	defer sysProxyMu.Unlock()
	if time.Since(sysProxyCacheAt) < sysProxyCacheTTL {
		return sysProxyCached
	}
	sysProxyCacheAt = time.Now()
	sysProxyCached = readSystemProxy()
	return sysProxyCached
}

func readSystemProxy() *neturl.URL {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return nil
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil {
		return nil
	}
	raw := strings.TrimSpace(server)
	if raw == "" {
		return nil
	}
	// ProxyServer is either "host:port" or per-scheme "http=…;https=…;socks=…".
	// HTTP(S) proxies work through the HTTP transport; socks entries are skipped.
	if strings.Contains(raw, "=") {
		best := ""
		for _, kv := range strings.Split(raw, ";") {
			scheme, addr, ok := strings.Cut(strings.TrimSpace(kv), "=")
			if !ok || addr == "" {
				continue
			}
			switch scheme {
			case "https":
				best = addr
			case "http":
				if best == "" {
					best = addr
				}
			}
		}
		raw = best
	}
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := neturl.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}
