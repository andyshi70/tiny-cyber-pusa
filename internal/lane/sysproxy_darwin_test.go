//go:build darwin

package lane

import "testing"

// readSystemProxy shells out to scutil, so the parsing is tested against the
// exact output shapes macOS produces rather than a live proxy configuration.
func TestReadSystemProxyShape(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want string // "" = no proxy
	}{
		{
			name: "http+https enabled prefers https",
			out: `<dictionary> {
  HTTPEnable : 1
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
  HTTPSEnable : 1
  HTTPSPort : 7890
  HTTPSProxy : 127.0.0.1
  ProxyAutoConfigEnable : 0
}</dictionary>`,
			want: "http://127.0.0.1:7890",
		},
		{
			name: "enabled entry with the port folded into the host",
			out: `<dictionary> {
  HTTPEnable : 1
  HTTPProxy : 127.0.0.1:8080
  HTTPPort : 0
}</dictionary>`,
			want: "http://127.0.0.1:8080",
		},
		{
			name: "proxy present but disabled must not be used",
			out: `<dictionary> {
  HTTPEnable : 0
  HTTPPort : 7890
  HTTPProxy : 127.0.0.1
}</dictionary>`,
			want: "",
		},
		{
			name: "listen-on-wildcard is not a usable proxy",
			out: `<dictionary> {
  HTTPEnable : 1
  HTTPProxy : 0.0.0.0
  HTTPPort : 7890
}</dictionary>`,
			want: "",
		},
		{
			name: "empty dictionary means no proxy",
			out:  `<dictionary> {\n}</dictionary>`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseScutilProxy(tc.out)
			if tc.want == "" {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("got nil, want %s", tc.want)
			}
			if got.String() != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
