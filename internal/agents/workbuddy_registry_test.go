package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zen-gate/internal/lane"
	"zen-gate/internal/store"
)

// TestRegistryWorkBuddyWiring exercises the full registry → adapter → file
// path with an isolated home, proving the adapter is registered and driven by
// Enable/Disable like the built-ins.
func TestRegistryWorkBuddyWiring(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	dir := wbConfig(t, "[]")

	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry(st)
	reg.SetEndpoints("http://127.0.0.1:8787/v1", []lane.ModelInfo{
		{ID: "deepseek-v4-flash-free", Name: "DeepSeek V4 Flash"},
	})

	var view *View
	for _, v := range reg.Views() {
		if v.ID == "workbuddy" {
			vv := v
			view = &vv
			break
		}
	}
	if view == nil {
		t.Fatal("Views() 中缺少 workbuddy")
	}
	if !view.Installed || view.Enabled {
		t.Fatalf("初始视图错误: %+v", view)
	}

	if err := reg.Enable("workbuddy"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if models, _ := doc["models"].([]any); len(models) != 1 {
		t.Fatalf("注册表 Enable 应写入 1 个模型，得到 %v", doc["models"])
	}
	if !st.Config().EnabledAgents["workbuddy"] {
		t.Fatal("启用状态应持久化到 store")
	}

	if err := reg.Disable("workbuddy"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(dir, "models.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Fatalf("注册表 Disable 应还原原文件，得到 %q", data)
	}
	if st.Config().EnabledAgents["workbuddy"] {
		t.Fatal("禁用后 store 状态应为 false")
	}
}
