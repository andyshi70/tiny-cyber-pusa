package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// qoder injects a custom provider into ~/.qoder-cn/settings.json (Qoder IDE).
// The IDE calls the provider's baseUrl directly from the local machine, so the
// 127.0.0.1 gateway works. Entries live under the "providers" key exactly as
// the IDE's own 添加模型 dialog writes them; they are self-identified by their
// 127.0.0.1 baseUrl, and the pristine file is backed up before the first write
// and restored verbatim on disable.

type qoder struct{}

func newQoder() *qoder { return &qoder{} }

func (q *qoder) Meta() (string, string, string) {
	return "qoder", "Qoder", "~/.qoder-cn/settings.json 注入 providers 自定义模型（IDE 本地直连）"
}

// configPath: the IDE's settings file. QODER_CN_CONFIG_DIR mirrors the env
// override convention the other adapters follow; the IDE itself always uses
// ~/.qoder-cn.
func (q *qoder) configPath() string {
	if p := os.Getenv("QODER_CN_CONFIG_DIR"); p != "" {
		return filepath.Join(p, "settings.json")
	}
	return homePath(".qoder-cn", "settings.json")
}

func (q *qoder) Detect() (bool, string, string) {
	dir := filepath.Dir(q.configPath())
	if _, err := os.Stat(dir); err != nil {
		return false, "", "未检测到 Qoder"
	}
	version := ""
	if data, err := os.ReadFile(filepath.Join(dir, ".qoder-app-status.json")); err == nil {
		var st struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(data, &st) == nil {
			version = st.Version
		}
	}
	detail := "检测到 ~/.qoder-cn"
	if _, err := os.Stat(q.configPath()); err != nil {
		detail = "检测到 ~/.qoder-cn（settings.json 尚未创建）"
	}
	return true, version, detail
}

func (q *qoder) IsEnabled() (bool, string, error) {
	doc, err := q.read()
	if err != nil {
		return false, "", nil
	}
	return q.ownProviders(doc) != nil, "", nil
}

// read parses settings.json. Unparseable JSON refuses the write (tier-2 rule).
func (q *qoder) read() (map[string]any, error) {
	data, err := os.ReadFile(q.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("settings.json 无法解析（%w）；zen-gate 拒绝写入", err)
	}
	return doc, nil
}

func (q *qoder) Enable(o Options) error {
	path := q.configPath()
	if raw, err := os.ReadFile(path); err == nil {
		// Re-enabling must not overwrite the pristine restore point with
		// already-injected content, so only back up files without our entries.
		doc, perr := q.parse(raw)
		if perr != nil || q.ownProviders(doc) == nil {
			_, _ = backupFile("qoder", path, raw)
		}
	}
	doc, err := q.read()
	if err != nil {
		return err
	}

	prov := q.providers(doc)
	// Drop previous zen-gate entries (and any manual localhost duplicate of
	// ours — the pristine copy stays in the backup), then add the full set.
	for k := range prov {
		if isLocalProviderEntry(prov[k]) {
			delete(prov, k)
		}
	}
	prov[qoderProviderKey] = q.buildEntry(o)
	doc["providers"] = prov

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	return atomicWrite(path, data)
}

func (q *qoder) Disable() error {
	path := q.configPath()
	if b := latestBackup("qoder", "settings.json"); b != "" {
		if data, err := os.ReadFile(b); err == nil {
			return atomicWrite(path, data)
		}
	}
	// No backup: strip our own entries only.
	doc, err := q.read()
	if err != nil {
		return nil
	}
	prov := q.providers(doc)
	if q.ownProviders(doc) == nil {
		return nil
	}
	for k := range prov {
		if isLocalProviderEntry(prov[k]) {
			delete(prov, k)
		}
	}
	if len(prov) == 0 {
		delete(doc, "providers")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

// --- settings.json helpers ---------------------------------------------------

// qoderProviderKey is deterministic so re-enabling replaces instead of duplicates.
const qoderProviderKey = "qoder-custom-zen-gate"

func (q *qoder) parse(data []byte) (map[string]any, error) {
	doc := map[string]any{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("settings.json 无法解析（%w）；zen-gate 拒绝写入", err)
	}
	return doc, nil
}

func (q *qoder) providers(doc map[string]any) map[string]any {
	if m, ok := doc["providers"].(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// isLocalProviderEntry: injected entries (and the user's manual duplicates of
// them) always point at the local gateway.
func isLocalProviderEntry(entry any) bool {
	em, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	url, _ := em["baseUrl"].(string)
	return strings.Contains(url, "127.0.0.1")
}

// ownProviders returns the providers map if it holds at least one zen-gate
// (localhost) entry, else nil.
func (q *qoder) ownProviders(doc map[string]any) map[string]any {
	prov := q.providers(doc)
	for _, v := range prov {
		if isLocalProviderEntry(v) {
			return prov
		}
	}
	return nil
}

// buildEntry mirrors the exact shape the IDE's 添加模型 dialog writes.
func (q *qoder) buildEntry(o Options) map[string]any {
	models := make([]any, 0, len(o.Models))
	for _, m := range o.Models {
		// 自定义供应商的模型没有目录元数据；0 交给 IDE 自己的对话框默认值。
		cw := m.ContextWindow
		if cw <= 0 {
			cw = 200000
		}
		mo := m.MaxOutput
		if mo <= 0 {
			mo = 8192
		}
		entry := map[string]any{
			"model":           m.ID,
			"displayName":     m.Name,
			"contextWindow":   cw,
			"maxOutputTokens": mo,
			"capabilities": map[string]any{
				"vision": m.Vision,
			},
		}
		if m.Reasoning {
			entry["capabilities"].(map[string]any)["thinking"] = map[string]any{
				"modes":                 []string{"enabled"},
				"supportsEffort":        true,
				"supportedEffortLevels": []string{"low", "medium", "high", "xhigh", "max"},
			}
		}
		models = append(models, entry)
	}
	def := ""
	if len(o.Models) > 0 {
		def = o.Models[0].ID
	}
	return map[string]any{
		"baseUrl":  o.BaseURL,
		"apiKey":   o.APIKey,
		"type":     "openai-compatible",
		"protocol": "openai",
		"authType": "bearer",
		"model":    def,
		"models":   models,
	}
}
