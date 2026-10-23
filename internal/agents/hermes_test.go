package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"zen-gate/internal/lane"
)

const hermesBaseConfig = `# 顶部注释必须保留
model:
  default: example-model
  provider: example-provider          # 原 provider 注释
providers:
  example-provider:
    api: https://api.example.com/v1
    key_env: EXAMPLE_PROVIDER_API_KEY
    default_model: example-model
agent:
  name: tester
`

func hermesHome(t *testing.T, config string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)
	t.Setenv("ZEN_GATE_HOME", filepath.Join(home, "zen-gate-data"))
	if config != "" {
		if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func hermesOptions() Options {
	return Options{
		BaseURL:      "http://127.0.0.1:8787/v1",
		APIKey:       "ofm-test-key",
		DefaultModel: "region-locked",
		Models: []lane.ModelInfo{
			{ID: "region-locked", Name: "Region Locked", RegionSensitive: true},
			{ID: "mimo-v2.6-flash-free", Name: "MiMo V2.6 Flash"},
		},
	}
}

func readYAMLMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("written file is not valid YAML: %v\n%s", err, data)
	}
	return doc
}

func TestHermesEnableInjectsProviderAndDefaultModel(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(home, "config.yaml")
	doc := readYAMLMap(t, path)

	providers, _ := doc["providers"].(map[string]any)
	zg, ok := providers["zen-gate"].(map[string]any)
	if !ok {
		t.Fatalf("providers.zen-gate missing: %v", providers)
	}
	if zg["api"] != "http://127.0.0.1:8787/v1" {
		t.Errorf("api = %v", zg["api"])
	}
	if zg["api_key"] != "ofm-test-key" {
		t.Errorf("api_key = %v", zg["api_key"])
	}
	if zg["default_model"] != "mimo-v2.6-flash-free" {
		t.Errorf("default_model = %v, want the first non-region-sensitive model", zg["default_model"])
	}
	if _, ok := providers["example-provider"]; !ok {
		t.Error("existing example provider was removed")
	}

	model, _ := doc["model"].(map[string]any)
	if model["provider"] != "zen-gate" {
		t.Errorf("model.provider = %v", model["provider"])
	}
	if model["default"] != "mimo-v2.6-flash-free" {
		t.Errorf("model.default = %v", model["default"])
	}

	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "# 顶部注释必须保留") {
		t.Errorf("user comment lost:\n%s", raw)
	}
	if b := latestBackup("hermes", "config.yaml"); b == "" {
		t.Error("no backup written before the injection")
	}
	if _, err := os.Stat(filepath.Join(home, "zen-gate-state.json")); err != nil {
		t.Errorf("model state file missing: %v", err)
	}
}

func TestHermesDisableRestoresModelAndKeepsUserContent(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(home, "config.yaml")
	doc := readYAMLMap(t, path)
	providers, _ := doc["providers"].(map[string]any)
	if _, ok := providers["zen-gate"]; ok {
		t.Error("providers.zen-gate survived disable")
	}
	if _, ok := providers["example-provider"]; !ok {
		t.Error("user provider was lost")
	}
	model, _ := doc["model"].(map[string]any)
	if model["provider"] != "example-provider" || model["default"] != "example-model" {
		t.Errorf("model not restored: %v", model)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "# 顶部注释必须保留") {
		t.Errorf("user comment lost:\n%s", raw)
	}
	if !strings.Contains(string(raw), "# 原 provider 注释") {
		t.Errorf("inline comment on model.provider not restored:\n%s", raw)
	}
	if _, err := os.Stat(filepath.Join(home, "zen-gate-state.json")); !os.IsNotExist(err) {
		t.Error("state file should be removed after disable")
	}
}

func TestHermesDisableReportsFileAndBackup(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	enabled := readFile(t, filepath.Join(home, "config.yaml"))

	changes, err := h.DisableWithReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one file", changes)
	}
	c := changes[0]
	if c.File != filepath.Join(home, "config.yaml") {
		t.Errorf("file = %q", c.File)
	}
	if c.Backup == "" {
		t.Fatal("no backup path reported")
	}
	if got := readFile(t, c.Backup); got != enabled {
		t.Errorf("backup content = %q, want the pre-disable config", got)
	}
}

func TestHermesEnableIsIdempotent(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	providers, _ := doc["providers"].(map[string]any)
	if len(providers) != 2 {
		t.Fatalf("providers = %v, want exactly example-provider + zen-gate", providers)
	}
	model, _ := doc["model"].(map[string]any)
	if model["provider"] != "zen-gate" {
		t.Errorf("model.provider = %v", model["provider"])
	}

	data, err := os.ReadFile(filepath.Join(home, "zen-gate-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state hermesModelState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if !state.ProviderSet || state.Provider != "example-provider" {
		t.Errorf("re-enable clobbered the original provider: %+v", state)
	}
	if !state.DefaultSet || state.Default != "example-model" {
		t.Errorf("re-enable clobbered the original default: %+v", state)
	}
}

func TestHermesEnableRefusesInvalidYAML(t *testing.T) {
	const broken = "model: [unclosed\n"
	home := hermesHome(t, broken)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err == nil {
		t.Fatal("Enable must refuse to rewrite unparseable YAML")
	}
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != broken {
		t.Errorf("invalid config was modified:\n%s", raw)
	}
}

func TestHermesDisableRemovesModelKeysThatWereAbsent(t *testing.T) {
	home := hermesHome(t, "providers:\n  example-provider:\n    api: https://example.com/v1\n")
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	if _, ok := doc["model"]; ok {
		t.Errorf("model block should be gone again, got %v", doc["model"])
	}
}

func TestHermesDisableRestoresDefaultWithoutProvider(t *testing.T) {
	home := hermesHome(t, "model:\n  default: keep-me\n")
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	model, _ := doc["model"].(map[string]any)
	if model["default"] != "keep-me" {
		t.Errorf("model.default = %v, want keep-me", model["default"])
	}
	if _, ok := model["provider"]; ok {
		t.Errorf("model.provider should stay absent, got %v", model)
	}
}

func TestHermesIgnoresEmptyStateFile(t *testing.T) {
	home := hermesHome(t, "model:\n  default: keep-me\n")
	if err := os.WriteFile(filepath.Join(home, "zen-gate-state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	model, _ := doc["model"].(map[string]any)
	if model["default"] != "keep-me" {
		t.Errorf("an empty state file made Disable drop model.default: %v", model)
	}
}

func TestHermesRecoversWhenStateFileIsLost(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, "zen-gate-state.json")); err != nil {
		t.Fatal(err)
	}
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	if _, ok := doc["model"]; ok {
		t.Errorf("our injected model block survived disable: %v", doc["model"])
	}
	providers, _ := doc["providers"].(map[string]any)
	if _, ok := providers["zen-gate"]; ok {
		t.Error("providers.zen-gate survived disable")
	}
}

func TestHermesRejectsMultiDocumentConfig(t *testing.T) {
	const multi = "a: 1\n---\nb: 2\n"
	home := hermesHome(t, multi)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err == nil {
		t.Fatal("Enable must refuse multi-document YAML")
	}
	if err := h.Disable(); err == nil {
		t.Fatal("Disable must report multi-document YAML instead of claiming success")
	}
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != multi {
		t.Errorf("multi-document config was modified:\n%s", raw)
	}
}

func TestHermesHandlesScalarModelSentinel(t *testing.T) {
	tests := []struct {
		name   string
		config string
		null   bool
	}{
		{name: "empty string", config: "model: \"\"\n"},
		{name: "null", config: "model: null\n", null: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := hermesHome(t, tc.config)
			h := newHermes()
			if err := h.Enable(hermesOptions()); err != nil {
				t.Fatalf("fresh Hermes %q must be supported: %v", tc.name, err)
			}
			if err := h.Disable(); err != nil {
				t.Fatal(err)
			}
			doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
			got, ok := doc["model"]
			if !ok {
				t.Fatalf("model sentinel was not restored: %v", doc)
			}
			if tc.null && got != nil {
				t.Errorf("model = %#v, want null", got)
			}
			if !tc.null && got != "" {
				t.Errorf("model = %#v, want an empty string", got)
			}
		})
	}
}

func TestHermesDisableReportsUnparseableConfig(t *testing.T) {
	const broken = "model: [unclosed\n"
	home := hermesHome(t, broken)
	if err := newHermes().Disable(); err == nil {
		t.Fatal("Disable must return an error for unparseable YAML")
	}
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != broken {
		t.Errorf("invalid config was modified:\n%s", raw)
	}
}

func TestHermesPreservesUserOwnedZenGateProvider(t *testing.T) {
	const userConfig = `model:
  provider: zen-gate
  default: user-model
providers:
  zen-gate:
    api: https://user.example/v1
    api_key: user-key
    default_model: user-model
`
	home := hermesHome(t, userConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	providers, _ := doc["providers"].(map[string]any)
	zg, _ := providers["zen-gate"].(map[string]any)
	if zg["api"] != "https://user.example/v1" || zg["api_key"] != "user-key" {
		t.Errorf("pre-existing providers.zen-gate was not restored: %v", zg)
	}
	model, _ := doc["model"].(map[string]any)
	if model["provider"] != "zen-gate" || model["default"] != "user-model" {
		t.Errorf("user model pointer was not restored: %v", model)
	}
}

func TestHermesTreatsNullProvidersAsEmptyMap(t *testing.T) {
	home := hermesHome(t, "model:\n  default: keep-me\nproviders:\n")
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	providers, _ := doc["providers"].(map[string]any)
	if _, ok := providers["zen-gate"].(map[string]any); !ok {
		t.Fatalf("providers.zen-gate missing: %v", providers)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}
	doc = readYAMLMap(t, filepath.Join(home, "config.yaml"))
	model, _ := doc["model"].(map[string]any)
	if model["default"] != "keep-me" {
		t.Errorf("model.default = %v, want keep-me", model["default"])
	}
	if _, ok := doc["providers"]; ok {
		t.Errorf("empty providers block should be removed: %v", doc["providers"])
	}
}

func TestHermesLeavesNamedProfilesAlone(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	profile := filepath.Join(home, "profiles", "work")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	const profileConfig = "model:\n  provider: work-provider\n"
	profilePath := filepath.Join(profile, "config.yaml")
	if err := os.WriteFile(profilePath, []byte(profileConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != profileConfig {
		t.Errorf("named profile was modified:\n%s", raw)
	}
}

func TestHermesKeepsCommentOnlyConfig(t *testing.T) {
	const comments = "# 只有注释\n"
	home := hermesHome(t, comments)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "# 只有注释") {
		t.Errorf("comment-only config lost its comment:\n%s", raw)
	}
}

func TestHermesSentinelKeepsModelKeysAddedAfterEnable(t *testing.T) {
	home := hermesHome(t, "model: \"\"\n")
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, "config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Hermes onboarding fills in extra model fields after the gateway is on.
	next := strings.Replace(string(raw), "  default: mimo-v2.6-flash-free\n", "  default: mimo-v2.6-flash-free\n  max_tokens: 32768\n", 1)
	if next == string(raw) {
		t.Fatalf("test setup: default line not found:\n%s", raw)
	}
	if err := os.WriteFile(path, []byte(next), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, path)
	model, _ := doc["model"].(map[string]any)
	if model["max_tokens"] != 32768 {
		t.Errorf("model.max_tokens added after enable was lost: %v", model)
	}
	if _, ok := model["provider"]; ok {
		t.Errorf("injected model.provider survived: %v", model)
	}
	if _, ok := model["default"]; ok {
		t.Errorf("injected model.default survived: %v", model)
	}
}

func TestHermesDisableWithoutStateCleansInjectedKeys(t *testing.T) {
	const injected = `model:
  provider: zen-gate
  default: mimo-v2.6-flash-free
providers:
  zen-gate:
    api: http://127.0.0.1:8787/v1
    api_key: ofm-test-key
    default_model: mimo-v2.6-flash-free
  keep:
    api: https://keep.example/v1
`
	home := hermesHome(t, injected)
	if err := newHermes().Disable(); err != nil {
		t.Fatal(err)
	}
	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	providers, _ := doc["providers"].(map[string]any)
	if _, ok := providers["zen-gate"]; ok {
		t.Error("providers.zen-gate survived disable without state")
	}
	if _, ok := providers["keep"]; !ok {
		t.Error("unrelated provider was lost")
	}
	if _, ok := doc["model"]; ok {
		t.Errorf("injected model block survived: %v", doc["model"])
	}
}

func TestHermesBackfillsUserProviderLeftByFailedWrite(t *testing.T) {
	const userConfig = `model:
  provider: zen-gate
  default: user-model
providers:
  zen-gate:
    api: https://user.example/v1
    api_key: user-key
    default_model: user-model
`
	home := hermesHome(t, userConfig)
	// State from an Enable whose config write failed: no ProviderNode yet.
	const staleState = `{"managed":true,"injected_default":"old-model"}`
	if err := os.WriteFile(filepath.Join(home, "zen-gate-state.json"), []byte(staleState), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}

	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	providers, _ := doc["providers"].(map[string]any)
	zg, _ := providers["zen-gate"].(map[string]any)
	if zg["api"] != "https://user.example/v1" {
		t.Errorf("user provider was not restored after the backfill: %v", providers)
	}
	model, _ := doc["model"].(map[string]any)
	if model["provider"] != "zen-gate" || model["default"] != "user-model" {
		t.Errorf("user model pointer was not restored: %v", model)
	}
}

func TestHermesRejectsDuplicateTopLevelKeys(t *testing.T) {
	const dup = "model:\n  default: one\nmodel:\n  default: two\n"
	home := hermesHome(t, dup)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err == nil {
		t.Fatal("Enable must refuse duplicate top-level keys")
	}
	if err := h.Disable(); err == nil {
		t.Fatal("Disable must report duplicate top-level keys")
	}
	raw, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != dup {
		t.Errorf("duplicate-key config was modified:\n%s", raw)
	}
}

func TestHermesDisableRestoresScalarTags(t *testing.T) {
	home := hermesHome(t, "model:\n  provider: null\n  default: 123\n")
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	if err := h.Disable(); err != nil {
		t.Fatal(err)
	}
	doc := readYAMLMap(t, filepath.Join(home, "config.yaml"))
	model, _ := doc["model"].(map[string]any)
	if model["provider"] != nil {
		t.Errorf("model.provider = %#v, want null", model["provider"])
	}
	if model["default"] != 123 {
		t.Errorf("model.default = %#v (%T), want int 123", model["default"], model["default"])
	}
}

func TestHermesConfigPathUsesHermesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)
	if got, want := newHermes().configPath(), filepath.Join(home, "config.yaml"); got != want {
		t.Fatalf("configPath = %q, want %q", got, want)
	}
}

func TestHermesDetectReportsDefaultProfile(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	installed, _, detail := newHermes().Detect()
	if !installed {
		t.Fatal("installed config should be detected")
	}
	if !strings.Contains(detail, "config.yaml") {
		t.Errorf("detail = %q", detail)
	}
	_ = home
}

func TestHermesEnableReportsFileAndBackup(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	var changes []FileChange
	opts := hermesOptions()
	opts.OnFileChange = func(c FileChange) { changes = append(changes, c) }
	if err := h.Enable(opts); err != nil {
		t.Fatal(err)
	}

	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one file", changes)
	}
	c := changes[0]
	if c.File != filepath.Join(home, "config.yaml") {
		t.Errorf("file = %q", c.File)
	}
	if c.Created {
		t.Error("an existing config was reported as created")
	}
	if c.Backup == "" {
		t.Fatal("no backup path reported")
	}
	if got := readFile(t, c.Backup); got != hermesBaseConfig {
		t.Errorf("backup content = %q, want the pre-enable config", got)
	}
}

func TestHermesEnableReportsCreatedConfig(t *testing.T) {
	home := hermesHome(t, "")
	var changes []FileChange
	opts := hermesOptions()
	opts.OnFileChange = func(c FileChange) { changes = append(changes, c) }
	if err := newHermes().Enable(opts); err != nil {
		t.Fatal(err)
	}

	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one file", changes)
	}
	c := changes[0]
	if !c.Created || c.Backup != "" {
		t.Errorf("new config report = %+v, want created with no backup", c)
	}
	if _, err := os.Stat(filepath.Join(home, "config.yaml")); err != nil {
		t.Errorf("config.yaml was not created: %v", err)
	}
}

func TestHermesEnableAbortsWhenBackupFails(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	backups := filepath.Join(os.Getenv("ZEN_GATE_HOME"), "backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		t.Fatal(err)
	}
	// A file where the per-adapter backup directory belongs makes MkdirAll fail.
	if err := os.WriteFile(filepath.Join(backups, "hermes"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := newHermes().Enable(hermesOptions()); err == nil {
		t.Fatal("Enable must fail when the backup cannot be written")
	}
	if got := readFile(t, filepath.Join(home, "config.yaml")); got != hermesBaseConfig {
		t.Errorf("config was modified without a backup:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(home, "zen-gate-state.json")); !os.IsNotExist(err) {
		t.Error("state was written without a backup")
	}
}

func TestHermesDisableAbortsWhenBackupFails(t *testing.T) {
	home := hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	enabled := readFile(t, filepath.Join(home, "config.yaml"))

	backupDir := BackupDir("hermes")
	if err := os.RemoveAll(backupDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backupDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := h.Disable(); err == nil {
		t.Fatal("Disable must fail when the backup cannot be written")
	}
	if got := readFile(t, filepath.Join(home, "config.yaml")); got != enabled {
		t.Errorf("config was modified without a backup:\n%s", got)
	}
}

func TestHermesReenableDoesNotStackBackups(t *testing.T) {
	hermesHome(t, hermesBaseConfig)
	h := newHermes()
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(BackupDir("hermes"))
	if err != nil {
		t.Fatal(err)
	}
	before := len(entries)
	if err := h.Enable(hermesOptions()); err != nil {
		t.Fatal(err)
	}
	entries, err = os.ReadDir(BackupDir("hermes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != before {
		t.Errorf("idempotent re-enable added %d backups", len(entries)-before)
	}
}
