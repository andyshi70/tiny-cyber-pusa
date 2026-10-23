package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zen-gate/internal/store"
)

// TestInjectableModelsMergesCustomProviders: the adapter-facing model set must
// include enabled custom-provider models as "<providerID>/<model>" (this is
// what puts 自定义 API models into agent pickers), respect the unchecked ids,
// and skip disabled providers. Regression for the NVIDIA-NIM-in-agents gap.
func TestInjectableModelsMergesCustomProviders(t *testing.T) {
	up := fakeUpstream(t, []string{`{"choices":[{"delta":{"content":"x"}}]}`, `data: [DONE]`})
	defer up.Close()
	s := newTestServer(t, up)

	cfg := s.Store.Config()
	cfg.Providers = append(cfg.Providers,
		store.Provider{
			ID: "nvidia-nim", Name: "NVIDIA NIM", BaseURL: "https://integrate.api.nvidia.com/v1",
			Protocol: "openai", Enabled: true,
			Models: []string{"deepseek-ai/deepseek-v4.1-flash", "nvidia/nemotron-3-ultra-550b-a55b"},
		},
		store.Provider{
			ID: "sleepy", Name: "Disabled", BaseURL: "https://x.example/v1",
			Protocol: "openai", Enabled: false,
			Models: []string{"hidden-model"},
		},
	)
	cfg.HiddenModels = []string{"nvidia-nim/nvidia/nemotron-3-ultra-550b-a55b"}

	ids := map[string]bool{}
	for _, m := range s.InjectableModels() {
		if ids[m.ID] {
			t.Fatalf("重复模型 %q", m.ID)
		}
		ids[m.ID] = true
	}
	if !ids["nvidia-nim/deepseek-ai/deepseek-v4.1-flash"] {
		t.Fatal("启用供应商的模型应进入注入清单")
	}
	if ids["nvidia-nim/nvidia/nemotron-3-ultra-550b-a55b"] {
		t.Fatal("被勾掉的自定义模型不应进入注入清单")
	}
	if ids["sleepy/hidden-model"] {
		t.Fatal("未启用供应商的模型不应进入注入清单")
	}
	nFree := 0
	for _, m := range s.VisibleModels() {
		nFree++
		_ = m
	}
	if len(ids) <= nFree {
		t.Fatalf("注入清单应比免费车道多出自定义模型: %d vs %d", len(ids), nFree)
	}
}

// TestCodexCatalogIncludesCustomModels: the picker's live-refresh endpoint
// must not drown custom-provider models back out.
func TestCodexCatalogIncludesCustomModels(t *testing.T) {
	up := fakeUpstream(t, []string{`{"choices":[{"delta":{"content":"x"}}]}`, `data: [DONE]`})
	defer up.Close()
	s := newTestServer(t, up)

	cfg := s.Store.Config()
	cfg.Providers = append(cfg.Providers, store.Provider{
		ID: "nvidia-nim", Name: "NVIDIA NIM", BaseURL: "https://integrate.api.nvidia.com/v1",
		Protocol: "openai", Enabled: true,
		Models: []string{"deepseek-ai/deepseek-v4.1-flash"},
	})

	rec := httptest.NewRecorder()
	s.handleCodexCatalog(rec, httptest.NewRequest(http.MethodGet, "/v1/codex-catalog", nil))
	if rec.Code != 200 {
		t.Fatalf("codex catalog status = %d", rec.Code)
	}
	var out struct {
		Models []struct {
			Slug        string `json:"slug"`
			Description string `json:"description"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range out.Models {
		if m.Slug == "nvidia-nim/deepseek-ai/deepseek-v4.1-flash" {
			found = true
			if !strings.Contains(m.Description, "NVIDIA NIM") {
				t.Fatalf("自定义模型描述应带供应商名，得到 %q", m.Description)
			}
		}
	}
	if !found {
		t.Fatal("codex 在线目录应包含自定义供应商模型")
	}
}
