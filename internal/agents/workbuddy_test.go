package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zen-gate/internal/lane"
)

func wbOptions() Options {
	return Options{
		BaseURL: "http://127.0.0.1:8787/v1",
		APIKey:  "ofm-test",
		Models: []lane.ModelInfo{
			{ID: "deepseek-v4-flash-free", Name: "DeepSeek V4 Flash", Vision: false},
			{ID: "mimo-v2.6-flash-free", Name: "Mimo Flash", Vision: true},
		},
		DefaultModel: "deepseek-v4-flash-free",
	}
}

func wbConfig(t *testing.T, initial string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".workbuddy")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WORKBUDDY_CONFIG_DIR", dir)
	if initial != "" {
		if err := os.WriteFile(filepath.Join(dir, "models.json"), []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func wbReadModel(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("models.json 不是对象: %v\n%s", err, data)
	}
	return doc
}

func agentBackupNames(t *testing.T, agentID string) []string {
	t.Helper()
	root := os.Getenv("ZEN_GATE_HOME")
	if root == "" {
		t.Fatal("ZEN_GATE_HOME 未设置")
	}
	dirEntries, err := os.ReadDir(filepath.Join(root, "backups", agentID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range dirEntries {
		names = append(names, e.Name())
	}
	return names
}

func wbBackups(t *testing.T) []string { return agentBackupNames(t, "workbuddy") }

func TestWorkBuddyDetect(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nothing")
	t.Setenv("WORKBUDDY_CONFIG_DIR", missing)
	w := newWorkBuddy()
	if installed, _, detail := w.Detect(); installed {
		t.Fatalf("空目录不应检测到安装: %s", detail)
	}

	dir := wbConfig(t, "[]")
	if err := os.WriteFile(filepath.Join(dir, "last-launch.json"), []byte(`{"version":"5.6.2"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	installed, version, detail := newWorkBuddy().Detect()
	if !installed || version != "5.6.2" {
		t.Fatalf("Detect = %v, %q", installed, version)
	}
	if detail == "" {
		t.Fatal("detail 不应为空")
	}
}

func TestWorkBuddyEnableDisableRoundTrip(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := wbConfig(t, "[]")
	w := newWorkBuddy()

	if err := w.Enable(wbOptions()); err != nil {
		t.Fatal(err)
	}
	doc := wbReadModel(t, dir)
	models, _ := doc["models"].([]any)
	if len(models) != 2 {
		t.Fatalf("期望 2 个模型条目，得到 %d", len(models))
	}
	first, _ := models[0].(map[string]any)
	if url, _ := first["url"].(string); url != "http://127.0.0.1:8787/v1/chat/completions" {
		t.Fatalf("url 应为完整端点，得到 %q", url)
	}
	// availableModels 是下拉框白名单，绝不能主动创建，否则 WorkBuddy 内置模型
	// 会被全部隐藏。
	if _, has := doc["availableModels"]; has {
		t.Fatalf("不应创建 availableModels 白名单，得到 %v", doc["availableModels"])
	}
	if enabled, _, _ := w.IsEnabled(); !enabled {
		t.Fatal("Enable 后 IsEnabled 应为 true")
	}

	// 原始 `[]` 应作为唯一（pristine）备份保存，重复 Enable 不产生新备份。
	if n := len(wbBackups(t)); n != 1 {
		t.Fatalf("首次 Enable 后应有 1 个备份，得到 %d", n)
	}
	if err := w.Enable(wbOptions()); err != nil {
		t.Fatal(err)
	}
	if n := len(wbBackups(t)); n != 1 {
		t.Fatalf("重复 Enable 不应新增备份，得到 %d", n)
	}

	if err := w.Disable(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("Disable 应原样还原为 []，得到 %q", data)
	}
	if enabled, _, _ := w.IsEnabled(); enabled {
		t.Fatal("Disable 后 IsEnabled 应为 false")
	}
}

func TestWorkBuddyPreservesUserModels(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := wbConfig(t, `{
  "models": [
    {"id": "my-model", "name": "Mine", "url": "https://api.example.com/v1/chat/completions", "apiKey": "sk-x"}
  ],
  "availableModels": ["my-model"],
  "futureKey": true
}`)
	w := newWorkBuddy()
	if err := w.Enable(wbOptions()); err != nil {
		t.Fatal(err)
	}
	doc := wbReadModel(t, dir)
	models, _ := doc["models"].([]any)
	if len(models) != 3 {
		t.Fatalf("用户模型应保留，共期望 3 条，得到 %d", len(models))
	}
	first, _ := models[0].(map[string]any)
	if id, _ := first["id"].(string); id != "my-model" {
		t.Fatalf("用户模型应排在最前，得到 %v", first)
	}
	if _, ok := doc["futureKey"]; !ok {
		t.Fatal("未知字段应原样保留")
	}
	avail, _ := doc["availableModels"].([]any)
	if len(avail) != 3 || avail[0] != "my-model" {
		t.Fatalf("availableModels 应保留用户项并追加 2 个，得到 %v", avail)
	}

	// 无备份的 Disable 只摘除 zen-gate 条目。
	if err := w.Disable(); err != nil {
		t.Fatal(err)
	}
	doc = wbReadModel(t, dir)
	models, _ = doc["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("Disable 后应只剩用户模型，得到 %d", len(models))
	}
	avail, _ = doc["availableModels"].([]any)
	if len(avail) != 1 || avail[0] != "my-model" {
		t.Fatalf("Disable 后 availableModels 应只剩用户项，得到 %v", avail)
	}
	if enabled, _, _ := w.IsEnabled(); enabled {
		t.Fatal("Disable 后不应再是启用态")
	}
}

func TestWorkBuddyAvailableModelsWhitelist(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	// 用户已有非空白名单 → 追加我们的 id；空白名单 → 原样保留，绝不变成白名单。
	dir := wbConfig(t, `{"models":[],"availableModels":[]}`)
	w := newWorkBuddy()
	if err := w.Enable(wbOptions()); err != nil {
		t.Fatal(err)
	}
	doc := wbReadModel(t, dir)
	if avail, _ := doc["availableModels"].([]any); len(avail) != 0 {
		t.Fatalf("空 availableModels 应保持为空，得到 %v", avail)
	}

	dir = wbConfig(t, `{"availableModels":["my-model"]}`)
	if err := w.Enable(wbOptions()); err != nil {
		t.Fatal(err)
	}
	doc = wbReadModel(t, dir)
	avail, _ := doc["availableModels"].([]any)
	if len(avail) != 3 || avail[0] != "my-model" {
		t.Fatalf("非空白名单应保留用户项并追加 2 个，得到 %v", avail)
	}
}

func TestWorkBuddyRefusesGarbage(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	wbConfig(t, "{not json")
	w := newWorkBuddy()
	if err := w.Enable(wbOptions()); err == nil {
		t.Fatal("无法解析的 models.json 应拒绝写入")
	}
}
