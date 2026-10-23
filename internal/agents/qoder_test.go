package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zen-gate/internal/lane"
	"zen-gate/internal/store"
)

func qoderOptions() Options {
	return Options{
		BaseURL: "http://127.0.0.1:8787/v1",
		APIKey:  "ofm-test",
		Models: []lane.ModelInfo{
			{ID: "mimo-v2.6-flash-free", Name: "MiMo V2.6 Flash", Vision: true, Reasoning: true, ContextWindow: 200000, MaxOutput: 8192},
			{ID: "space-bunny-free", Name: "Space Bunny", ContextWindow: 128000, MaxOutput: 4096},
		},
		DefaultModel: "mimo-v2.6-flash-free",
	}
}

func qoderConfig(t *testing.T, initial string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".qoder-cn")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QODER_CN_CONFIG_DIR", dir)
	if initial != "" {
		if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func qoderReadDoc(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings.json 不是对象: %v\n%s", err, data)
	}
	return doc
}

func TestQoderDetect(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nothing")
	t.Setenv("QODER_CN_CONFIG_DIR", missing)
	if installed, _, _ := newQoder().Detect(); installed {
		t.Fatal("空目录不应检测到安装")
	}

	dir := qoderConfig(t, "")
	if err := os.WriteFile(filepath.Join(dir, ".qoder-app-status.json"), []byte(`{"version":"0.4.3","product":"qodercn"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	installed, version, _ := newQoder().Detect()
	if !installed || version != "0.4.3" {
		t.Fatalf("Detect = %v, %q", installed, version)
	}
}

func TestQoderEnableDisableRoundTrip(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := qoderConfig(t, `{"permissions":{"trustDirectories":[]}}`)
	w := newQoder()

	if err := w.Enable(qoderOptions()); err != nil {
		t.Fatal(err)
	}
	doc := qoderReadDoc(t, dir)
	prov, _ := doc["providers"].(map[string]any)
	if len(prov) != 1 {
		t.Fatalf("应注入 1 个 provider，得到 %v", prov)
	}
	entry, _ := prov["qoder-custom-zen-gate"].(map[string]any)
	if entry == nil {
		t.Fatal("缺少 qoder-custom-zen-gate 条目")
	}
	if url, _ := entry["baseUrl"].(string); url != "http://127.0.0.1:8787/v1" {
		t.Fatalf("baseUrl 错误: %q", url)
	}
	if typ, _ := entry["type"].(string); typ != "openai-compatible" {
		t.Fatalf("type 应为 openai-compatible，得到 %q", typ)
	}
	models, _ := entry["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("应含 2 个模型，得到 %d", len(models))
	}
	first, _ := models[0].(map[string]any)
	caps, _ := first["capabilities"].(map[string]any)
	if caps["vision"] != true {
		t.Fatal("推理模型应带 vision=true")
	}
	if _, has := caps["thinking"]; !has {
		t.Fatal("reasoning 模型应带 thinking 块")
	}
	second, _ := models[1].(map[string]any)
	caps2, _ := second["capabilities"].(map[string]any)
	if _, has := caps2["thinking"]; has {
		t.Fatal("非 reasoning 模型不应带 thinking 块")
	}
	if enabled, _, _ := w.IsEnabled(); !enabled {
		t.Fatal("Enable 后 IsEnabled 应为 true")
	}
	// 用户的原有键原样保留
	if _, ok := doc["permissions"]; !ok {
		t.Fatal("settings.json 原有内容应保留")
	}

	// 原始文件应作为唯一 pristine 备份，重复 Enable 不新增备份
	if n := len(agentBackupNames(t, "qoder")); n != 1 {
		t.Fatalf("首次 Enable 后应有 1 个备份，得到 %d", n)
	}
	if err := w.Enable(qoderOptions()); err != nil {
		t.Fatal(err)
	}
	if n := len(agentBackupNames(t, "qoder")); n != 1 {
		t.Fatalf("重复 Enable 不应新增备份，得到 %d", n)
	}

	if err := w.Disable(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"permissions\":{\"trustDirectories\":[]}}" {
		t.Fatalf("Disable 应原样还原，得到 %q", data)
	}
	if enabled, _, _ := w.IsEnabled(); enabled {
		t.Fatal("Disable 后 IsEnabled 应为 false")
	}
}

func TestQoderPreservesUserProviders(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := qoderConfig(t, `{
  "providers": {
    "qoder-custom-user-one": {
      "baseUrl": "https://api.example.com/v1",
      "apiKey": "sk-x",
      "type": "openai-compatible",
      "protocol": "openai",
      "authType": "bearer",
      "model": "gpt-x",
      "models": [{"model": "gpt-x", "displayName": "GPT X"}]
    }
  }
}`)
	w := newQoder()
	if err := w.Enable(qoderOptions()); err != nil {
		t.Fatal(err)
	}
	doc := qoderReadDoc(t, dir)
	prov, _ := doc["providers"].(map[string]any)
	if len(prov) != 2 {
		t.Fatalf("用户的 provider 应保留，共期望 2 个，得到 %v", prov)
	}
	if _, ok := prov["qoder-custom-user-one"]; !ok {
		t.Fatal("用户 provider 不应被删除")
	}

	// 无备份的 Disable 只摘除 zen-gate 条目
	if err := w.Disable(); err != nil {
		t.Fatal(err)
	}
	doc = qoderReadDoc(t, dir)
	prov, _ = doc["providers"].(map[string]any)
	if len(prov) != 1 {
		t.Fatalf("Disable 后应只剩用户 provider，得到 %v", prov)
	}
	if _, ok := prov["qoder-custom-user-one"]; !ok {
		t.Fatal("用户 provider 应原样保留")
	}
}

func TestQoderRefusesGarbage(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	qoderConfig(t, "{not json")
	if err := newQoder().Enable(qoderOptions()); err == nil {
		t.Fatal("无法解析的 settings.json 应拒绝写入")
	}
}

func TestRegistryQoderWiring(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := qoderConfig(t, "{}")

	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(st)
	reg.SetEndpoints("http://127.0.0.1:8787/v1", []lane.ModelInfo{
		{ID: "mimo-v2.6-flash-free", Name: "MiMo V2.6 Flash"},
	})

	found := false
	for _, v := range reg.Views() {
		if v.ID == "qoder" {
			found = true
			if !v.Installed {
				t.Fatal("qoder 应被检测为已安装")
			}
		}
	}
	if !found {
		t.Fatal("Views() 中缺少 qoder")
	}

	if err := reg.Enable("qoder"); err != nil {
		t.Fatal(err)
	}
	doc := qoderReadDoc(t, dir)
	prov, _ := doc["providers"].(map[string]any)
	if _, ok := prov["qoder-custom-zen-gate"]; !ok {
		t.Fatalf("注册表 Enable 应写入 provider，得到 %v", prov)
	}
	if !st.Config().EnabledAgents["qoder"] {
		t.Fatal("启用状态应持久化到 store")
	}

	if err := reg.Disable("qoder"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("注册表 Disable 应还原原文件，得到 %q", data)
	}
}
