package subs

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// HTTPDoer is the minimal client surface FetchSubscription needs; *http.Client
// satisfies it.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func splitHostPort(hostport string) (string, string, error) {
	if h, p, err := net.SplitHostPort(hostport); err == nil {
		return h, p, nil
	}
	// Bare host:port (no brackets) fails SplitHostPort for IPv6; the last
	// colon covers the common plain case.
	i := strings.LastIndex(hostport, ":")
	if i <= 0 || i == len(hostport)-1 {
		return "", "", fmt.Errorf("缺少端口: %q", hostport)
	}
	return hostport[:i], hostport[i+1:], nil
}

func atoiOr(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return n
	}
	return def
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func shortHash(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:4])
}

// decodeAnyBase64 tries the padded and raw variants of the standard and
// websafe alphabets — subscription generators use all of them.
func decodeAnyBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("空 base64")
	}
	trimmed := strings.TrimRight(s, "=")
	for _, enc := range []*base64.Encoding{
		base64.RawStdEncoding, base64.RawURLEncoding,
	} {
		// Some generators swap +/ and -/ carelessly; try the given alphabet
		// first, then its counterpart with the other alphabet's specials.
		if dec, err := enc.DecodeString(trimmed); err == nil {
			return dec, nil
		}
	}
	// Padded standard alphabets, padding repaired.
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding} {
		padded := trimmed + strings.Repeat("=", (4-len(trimmed)%4)%4)
		if dec, err := enc.DecodeString(padded); err == nil {
			return dec, nil
		}
	}
	return nil, fmt.Errorf("不是有效的 base64")
}
