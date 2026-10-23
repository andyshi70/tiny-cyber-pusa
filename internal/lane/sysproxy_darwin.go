//go:build darwin

package lane

import (
	neturl "net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// System proxy support: on macOS the equivalent of the Windows WinINET
// settings is the SystemConfiguration proxy dictionary, which `scutil --proxy`
// prints as `key : value` lines. Proxy clients (ClashX / Surge / …) write it
// when the user enables 系统代理 — again NOT through environment variables,
// which is why the "env" mode never sees them.
//
// The result is cached for a few seconds so flipping the proxy in the client's
// UI is picked up without restarting zen-gate.

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

// readSystemProxy shells out to scutil and parses the reply.
func readSystemProxy() *neturl.URL {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return nil
	}
	return parseScutilProxy(string(out))
}

// parseScutilProxy turns `scutil --proxy` output into a proxy URL, preferring
// the HTTPS entry — that is the transport the free lane's own upstream uses.
func parseScutilProxy(out string) *neturl.URL {
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	for _, scheme := range []string{"HTTPS", "HTTP"} {
		if kv[scheme+"Enable"] != "1" {
			continue // not enabled — a stale entry must not win
		}
		host, port := kv[scheme+"Proxy"], kv[scheme+"Port"]
		// Some tools put the port in the host field; tolerate both shapes.
		if h, p, ok := strings.Cut(host, ":"); ok {
			host, port = h, p
		}
		if host == "" || port == "" || host == "0.0.0.0" || port == "0" {
			continue
		}
		u, err := neturl.Parse("http://" + host + ":" + port)
		if err != nil || u.Host == "" {
			continue
		}
		return u
	}
	return nil
}
