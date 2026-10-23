package subs

import (
	"encoding/json"
	"testing"
)

func TestBuildConfigLayout(t *testing.T) {
	nodes := []Node{
		{ID: "a", Name: "A", Proto: "vless", Outbound: map[string]any{"type": "vless", "server": "a.example.com", "server_port": 443}},
		{ID: "b", Name: "B", Proto: "ss", Outbound: map[string]any{"type": "shadowsocks", "server": "b.example.com", "server_port": 8388}},
		{ID: "c", Name: "C", Proto: "hy2", Outbound: map[string]any{"type": "hysteria2", "server": "c.example.com", "server_port": 8443}},
	}
	data, err := BuildConfig(nodes)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Inbounds []struct {
			Tag        string `json:"tag"`
			ListenPort int    `json:"listen_port"`
		} `json:"inbounds"`
		Outbounds []struct {
			Tag  string `json:"tag"`
			Type string `json:"type"`
		} `json:"outbounds"`
		Route struct {
			Rules []struct {
				Inbound  []string `json:"inbound"`
				Outbound string   `json:"outbound"`
			} `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inbounds) != 3 || len(cfg.Outbounds) != 3 || len(cfg.Route.Rules) != 3 {
		t.Fatalf("inbound/outbound/rule 数量错误: %d/%d/%d", len(cfg.Inbounds), len(cfg.Outbounds), len(cfg.Route.Rules))
	}
	for i, want := range []int{PortBase, PortBase + 1, PortBase + 2} {
		if cfg.Inbounds[i].ListenPort != want {
			t.Fatalf("inbound %d 端口 = %d, 期望 %d", i, cfg.Inbounds[i].ListenPort, want)
		}
		r := cfg.Route.Rules[i]
		if len(r.Inbound) != 1 || r.Inbound[0] != cfg.Inbounds[i].Tag || r.Outbound != cfg.Outbounds[i].Tag {
			t.Fatalf("rule %d 未绑定 inbound→outbound: %+v", i, r)
		}
	}
	if cfg.Outbounds[1].Type != "shadowsocks" {
		t.Fatalf("outbound 1 类型错误: %s", cfg.Outbounds[1].Type)
	}
}

func TestBuildConfigCapsNodes(t *testing.T) {
	nodes := make([]Node, MaxNodes+10)
	for i := range nodes {
		nodes[i] = Node{ID: "n", Outbound: map[string]any{"type": "socks", "server": "x", "server_port": 1}}
	}
	data, err := BuildConfig(nodes)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Inbounds []any `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inbounds) != MaxNodes {
		t.Fatalf("应截断到 %d 个节点，得到 %d", MaxNodes, len(cfg.Inbounds))
	}
}

func TestBuildConfigRejectsEmpty(t *testing.T) {
	if _, err := BuildConfig(nil); err == nil {
		t.Fatal("空节点应报错")
	}
}
