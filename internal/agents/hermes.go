package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// hermes injects an OpenAI-compatible provider into Hermes Agent's default
// profile (~/.hermes/config.yaml). Only the default profile is touched —
// named profiles under ~/.hermes/profiles keep their own config. The provider
// block is providers.zen-gate; Hermes discovers its models live from /models,
// so no model roster is written. The active model is switched to the gateway
// and the user's previous model.provider/model.default are stashed for
// Disable.
//
// The YAML is edited as a node tree so comments, key order and unrelated
// providers survive the round-trip.

type hermes struct{}

// hermesModelState records what was in config.yaml before the first Enable:
// the active model values, a pre-existing providers.zen-gate block, and a
// scalar/null "model:" sentinel that gets replaced by a mapping.
// Managed distinguishes a state written by this adapter from a missing,
// empty or truncated file. ProviderSet/DefaultSet distinguish an absent key
// from an empty one; comments are restored together with their value.
type hermesModelState struct {
	Managed bool `json:"managed"`

	ProviderSet  bool   `json:"provider_set"`
	Provider     string `json:"provider"`
	ProviderTag  string `json:"provider_tag,omitempty"`
	ProviderLine string `json:"provider_line_comment,omitempty"`

	DefaultSet  bool   `json:"default_set"`
	Default     string `json:"default"`
	DefaultTag  string `json:"default_tag,omitempty"`
	DefaultLine string `json:"default_line_comment,omitempty"`

	ProviderNode    string `json:"provider_node,omitempty"`
	InjectedDefault string `json:"injected_default,omitempty"`

	ModelSentinelSet   bool   `json:"model_sentinel_set,omitempty"`
	ModelSentinelTag   string `json:"model_sentinel_tag,omitempty"`
	ModelSentinelValue string `json:"model_sentinel_value,omitempty"`
	ModelSentinelLine  string `json:"model_sentinel_line_comment,omitempty"`
}

const hermesProviderID = "zen-gate"

func newHermes() *hermes { return &hermes{} }

func (h *hermes) Meta() (string, string, string) {
	return "hermes", "Hermes Agent", "在 default profile 的 config.yaml 注入 providers.zen-gate（HERMES_HOME 或 ~/.hermes，关闭即还原）"
}

// configPath follows Hermes' own resolution: HERMES_HOME, then the platform
// default. Only the default profile is targeted; named profiles are ignored.
func (h *hermes) configPath() string {
	if home := os.Getenv("HERMES_HOME"); home != "" {
		return filepath.Join(home, "config.yaml")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			return filepath.Join(appData, "hermes", "config.yaml")
		}
	}
	return homePath(".hermes", "config.yaml")
}

func (h *hermes) statePath() string {
	return filepath.Join(filepath.Dir(h.configPath()), "zen-gate-state.json")
}

func (h *hermes) Detect() (bool, string, string) {
	if _, err := os.Stat(h.configPath()); err == nil {
		return true, "", "检测到 config.yaml（default profile）"
	}
	if _, err := os.Stat(filepath.Dir(h.configPath())); err == nil {
		return true, "", "检测到 Hermes 配置目录（config.yaml 尚未创建）"
	}
	return false, "", "未检测到 Hermes Agent"
}

func (h *hermes) IsEnabled() (bool, string, error) {
	doc, _, err := h.read()
	if err != nil {
		return false, "", nil
	}
	providers := mapValue(doc.Content[0], "providers")
	return providers != nil && providers.Kind == yaml.MappingNode && mapValue(providers, hermesProviderID) != nil, "", nil
}

// read loads config.yaml as a YAML document whose body is a mapping. Missing
// and empty files yield an empty mapping; unparseable, multi-document or
// non-mapping files are refused rather than overwritten.
func (h *hermes) read() (*yaml.Node, []byte, error) {
	data, err := os.ReadFile(h.configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return emptyYAMLDocument(), nil, nil
		}
		return nil, nil, err
	}
	doc, err := decodeOneYAMLDocument(data)
	if err != nil {
		return nil, data, fmt.Errorf("config.yaml 无法解析（%w）；zen-gate 拒绝写入", err)
	}
	if doc.Kind == 0 {
		doc = *emptyYAMLDocument()
		if head := leadingComments(data); head != "" {
			doc.Content[0].HeadComment = head
		}
		return &doc, data, nil
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, data, fmt.Errorf("config.yaml 顶层不是 mapping；zen-gate 拒绝写入")
	}
	if key := duplicateKey(doc.Content[0]); key != "" {
		// PyYAML keeps the last duplicate; editing the first would report
		// success while Hermes reads the other one.
		return nil, data, fmt.Errorf("config.yaml 顶层存在重复键 %q；zen-gate 拒绝写入", key)
	}
	return &doc, data, nil
}

func duplicateKey(m *yaml.Node) string {
	seen := make(map[string]bool, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		key := m.Content[i].Value
		if seen[key] {
			return key
		}
		seen[key] = true
	}
	return ""
}

// decodeOneYAMLDocument rejects trailing "---" documents instead of silently
// rewriting only the first one.
func decodeOneYAMLDocument(data []byte) (yaml.Node, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return yaml.Node{}, nil
		}
		return yaml.Node{}, err
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case errors.Is(err, io.EOF):
		return doc, nil
	case err == nil:
		return yaml.Node{}, fmt.Errorf("包含多个 YAML 文档；zen-gate 拒绝写入")
	default:
		return yaml.Node{}, err
	}
}

func emptyYAMLDocument() *yaml.Node {
	return &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
}

// preserveLeadingComments keeps the file's top comment block even when the
// key it was attached to (usually the first one) is deleted. yaml.v3 attaches
// a leading comment to the first key node, so deleting every key drops it.
func preserveLeadingComments(raw, out []byte) []byte {
	head := leadingComments(raw)
	if head == "" || bytes.Contains(out, []byte(head)) {
		return out
	}
	return append([]byte(head+"\n"), out...)
}

// leadingComments keeps a comment-only config's text when a mapping has to be
// created around it.
func leadingComments(data []byte) string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "#") {
			break
		}
		out = append(out, t)
	}
	return strings.Join(out, "\n")
}

func (h *hermes) loadState() (hermesModelState, bool) {
	var state hermesModelState
	data, err := os.ReadFile(h.statePath())
	if err != nil {
		return state, false
	}
	if json.Unmarshal(data, &state) != nil {
		return hermesModelState{}, false
	}
	// Managed is the marker for states written by this version; the field
	// checks also accept a state from the first implementation.
	if (state.Managed && state.InjectedDefault != "") || state.ProviderSet || state.DefaultSet || state.ProviderNode != "" || state.ModelSentinelSet {
		return state, true
	}
	return hermesModelState{}, false
}

// defaultModel keeps region-gated models off the active slot, mirroring Codex.
func defaultModel(o Options) string {
	for _, m := range o.Models {
		if !m.RegionSensitive {
			return m.ID
		}
	}
	if o.DefaultModel != "" {
		return o.DefaultModel
	}
	for _, m := range o.Models {
		if m.ID != "" {
			return m.ID
		}
	}
	return ""
}

// providerIsOurs recognises the block Enable writes (endpoint + key + exactly
// our three fields), so a config left behind after a lost state file can still
// be disabled safely.
func (h *hermes) providerIsOurs(n *yaml.Node, o Options) bool {
	if n == nil || n.Kind != yaml.MappingNode || len(n.Content) != 6 {
		return false
	}
	api, key := mapValue(n, "api"), mapValue(n, "api_key")
	return api != nil && api.Kind == yaml.ScalarNode && api.Value == o.BaseURL &&
		key != nil && key.Kind == yaml.ScalarNode && key.Value == o.APIKey
}

func nodeToYAML(n *yaml.Node) (string, error) {
	b, err := marshalYAML(n)
	return string(b), err
}

func nodeFromYAML(s string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		return nil, err
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0], nil
	}
	if doc.Kind == 0 {
		return nil, fmt.Errorf("节点为空")
	}
	return &doc, nil
}

func isNullNode(n *yaml.Node) bool {
	return n != nil && n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// sentinelReplaceable reports whether model still holds only the two keys
// Enable added, so restoring the original scalar cannot swallow new fields.
func sentinelReplaceable(m *yaml.Node) bool {
	if m == nil {
		return true
	}
	if m.Kind != yaml.MappingNode || len(m.Content) != 4 {
		return false
	}
	p := mapValue(m, "provider")
	return p != nil && p.Value == hermesProviderID && mapValue(m, "default") != nil
}

func savedScalar(tag, value, line string) *yaml.Node {
	if tag == "" {
		tag = "!!str"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value, LineComment: line}
}

func (h *hermes) Enable(o Options) error {
	def := defaultModel(o)
	if def == "" {
		return fmt.Errorf("模型清单尚未就绪，无法设置 Hermes 默认模型")
	}

	doc, raw, err := h.read()
	if err != nil {
		return err
	}
	root := doc.Content[0]
	state, hasState := h.loadState()

	providers := mapValue(root, "providers")
	if providers == nil || isNullNode(providers) {
		providers = &yaml.Node{Kind: yaml.MappingNode}
		setMapKey(root, "providers", providers)
	} else if providers.Kind != yaml.MappingNode {
		return fmt.Errorf("config.yaml 的 providers 不是 mapping；zen-gate 拒绝写入")
	}
	existingProvider := mapValue(providers, hermesProviderID)

	// Hermes ships "model: ''" on a fresh install; a scalar is a sentinel,
	// not an error. Keep it so Disable can put the file back as it was.
	model := mapValue(root, "model")
	if model != nil && model.Kind != yaml.MappingNode {
		if model.Kind != yaml.ScalarNode {
			return fmt.Errorf("config.yaml 的 model 不是 mapping 或标量；zen-gate 拒绝写入")
		}
		if !hasState {
			state.ModelSentinelSet = true
			state.ModelSentinelTag = model.Tag
			state.ModelSentinelValue = model.Value
			state.ModelSentinelLine = model.LineComment
		}
		model = &yaml.Node{Kind: yaml.MappingNode}
		setMapKey(root, "model", model)
	} else if model == nil {
		model = &yaml.Node{Kind: yaml.MappingNode}
		setMapKey(root, "model", model)
	}

	// Stash the originals only on the first enable. Without a state file the
	// block Enable writes is treated as ours; anything else is preserved and
	// restored on Disable.
	stateChanged := !hasState
	if !hasState {
		state.Managed = true
	}
	// A config write can fail after the state file landed, so state may exist
	// while the file still carries the user's block. Stash it now.
	if state.ProviderNode == "" && existingProvider != nil && !h.providerIsOurs(existingProvider, o) {
		encoded, err := nodeToYAML(existingProvider)
		if err != nil {
			return fmt.Errorf("无法暂存原有 providers.%s 配置: %w", hermesProviderID, err)
		}
		state.ProviderNode = encoded
		stateChanged = true
		if hasState {
			// The user may have pointed the active model at that block after
			// the previous (failed) write; keep the pointer too.
			if !state.ProviderSet {
				if p := mapValue(model, "provider"); p != nil && p.Kind == yaml.ScalarNode && p.Value == hermesProviderID {
					state.ProviderSet, state.Provider = true, p.Value
					state.ProviderTag, state.ProviderLine = p.Tag, p.LineComment
				}
			}
			if !state.DefaultSet {
				if d := mapValue(model, "default"); d != nil && d.Kind == yaml.ScalarNode {
					state.DefaultSet, state.Default = true, d.Value
					state.DefaultTag, state.DefaultLine = d.Tag, d.LineComment
				}
			}
		}
	}
	if !hasState {
		userProvider := state.ProviderNode != ""

		prov := mapValue(model, "provider")
		if prov != nil {
			if prov.Kind != yaml.ScalarNode {
				return fmt.Errorf("config.yaml 的 model.provider 不是标量；zen-gate 拒绝写入")
			}
			if userProvider || prov.Value != hermesProviderID {
				state.ProviderSet, state.Provider = true, prov.Value
				state.ProviderTag, state.ProviderLine = prov.Tag, prov.LineComment
			}
		}

		oursPointer := !userProvider && prov != nil && prov.Value == hermesProviderID
		if d := mapValue(model, "default"); d != nil {
			if d.Kind != yaml.ScalarNode {
				return fmt.Errorf("config.yaml 的 model.default 不是标量；zen-gate 拒绝写入")
			}
			if !(oursPointer && d.Value == def) {
				state.DefaultSet, state.Default = true, d.Value
				state.DefaultTag, state.DefaultLine = d.Tag, d.LineComment
			}
		}
	}
	if state.InjectedDefault != def {
		state.InjectedDefault = def
		stateChanged = true
	}

	provider := &yaml.Node{Kind: yaml.MappingNode}
	setMapKey(provider, "api", strNode(o.BaseURL))
	setMapKey(provider, "api_key", strNode(o.APIKey))
	setMapKey(provider, "default_model", strNode(def))
	setMapKey(providers, hermesProviderID, provider)
	setMapKey(model, "provider", strNode(hermesProviderID))
	setMapKey(model, "default", strNode(def))

	out, err := marshalYAML(doc)
	if err != nil {
		return err
	}
	out = preserveLeadingComments(raw, out)
	if raw != nil && bytes.Equal(raw, out) {
		if stateChanged {
			return h.saveState(state)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(h.configPath()), 0o700); err != nil {
		return err
	}
	// The backup must exist before anything on disk changes: no restore point,
	// no write. A missing config has nothing to back up.
	backup, err := backupBeforeWriteStrict("hermes", h.configPath())
	if err != nil {
		return err
	}
	if stateChanged {
		if err := h.saveState(state); err != nil {
			return err
		}
	}
	if err := atomicWrite(h.configPath(), out); err != nil {
		return err
	}
	if o.OnFileChange != nil {
		o.OnFileChange(FileChange{File: h.configPath(), Backup: backup, Created: raw == nil})
	}
	return nil
}

func (h *hermes) saveState(state hermesModelState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicWrite(h.statePath(), b)
}

func (h *hermes) Disable() error {
	_, err := h.DisableWithReport()
	return err
}

// DisableWithReport restores the pre-enable model values and reports the
// restore point made before the rewrite.
func (h *hermes) DisableWithReport() ([]FileChange, error) {
	doc, raw, err := h.read()
	if err != nil {
		return nil, err // unparseable: report instead of claiming success
	}
	if raw == nil {
		// No config file: nothing to restore, just forget the stash.
		_ = os.Remove(h.statePath())
		return nil, nil
	}
	root := doc.Content[0]
	state, hasState := h.loadState()
	changed := false

	providers := mapValue(root, "providers")
	if isNullNode(providers) {
		providers = nil
	}
	injectedDefault := ""
	if hasState && state.ProviderNode != "" {
		if providers == nil || providers.Kind != yaml.MappingNode {
			if providers != nil && !isNullNode(providers) {
				return nil, fmt.Errorf("config.yaml 的 providers 不是 mapping，无法还原原配置")
			}
			providers = &yaml.Node{Kind: yaml.MappingNode}
			setMapKey(root, "providers", providers)
		}
		n, err := nodeFromYAML(state.ProviderNode)
		if err != nil {
			return nil, fmt.Errorf("无法读取暂存的 providers.%s 配置: %w", hermesProviderID, err)
		}
		setMapKey(providers, hermesProviderID, n)
		changed = true
	} else if providers != nil && providers.Kind == yaml.MappingNode {
		if p := mapValue(providers, hermesProviderID); p != nil {
			if dm := mapValue(p, "default_model"); dm != nil {
				injectedDefault = dm.Value
			}
			deleteMapKey(providers, hermesProviderID)
			changed = true
			if len(providers.Content) == 0 {
				deleteMapKey(root, "providers")
			}
		}
	}

	model := mapValue(root, "model")
	if hasState {
		if state.ModelSentinelSet {
			switch {
			case sentinelReplaceable(model):
				sentinel := &yaml.Node{
					Kind:        yaml.ScalarNode,
					Tag:         state.ModelSentinelTag,
					Value:       state.ModelSentinelValue,
					LineComment: state.ModelSentinelLine,
				}
				setMapKey(root, "model", sentinel)
				changed = true
			case model != nil && model.Kind == yaml.MappingNode:
				// Onboarding added keys after Enable; keep them and remove
				// only the two keys this adapter wrote.
				if p := mapValue(model, "provider"); p != nil && p.Value == hermesProviderID {
					if d := mapValue(model, "default"); d != nil && state.InjectedDefault != "" && d.Value == state.InjectedDefault {
						deleteMapKey(model, "default")
					}
					deleteMapKey(model, "provider")
					changed = true
				}
			}
		} else {
			if model == nil && (state.ProviderSet || state.DefaultSet) {
				model = &yaml.Node{Kind: yaml.MappingNode}
				setMapKey(root, "model", model)
			}
			if model != nil && model.Kind == yaml.MappingNode {
				if state.ProviderSet {
					setMapKey(model, "provider", savedScalar(state.ProviderTag, state.Provider, state.ProviderLine))
					changed = true
				} else if mapValue(model, "provider") != nil {
					deleteMapKey(model, "provider")
					changed = true
				}
				if state.DefaultSet {
					setMapKey(model, "default", savedScalar(state.DefaultTag, state.Default, state.DefaultLine))
					changed = true
				} else if mapValue(model, "default") != nil {
					deleteMapKey(model, "default")
					changed = true
				}
				if len(model.Content) == 0 {
					deleteMapKey(root, "model")
				}
			}
		}
	} else if model != nil && model.Kind == yaml.MappingNode {
		// No stash: never leave model.provider pointing at the provider we
		// just removed, and drop a default that is clearly the injected one.
		if p := mapValue(model, "provider"); p != nil && p.Value == hermesProviderID {
			if d := mapValue(model, "default"); d != nil && injectedDefault != "" && d.Value == injectedDefault {
				deleteMapKey(model, "default")
			}
			deleteMapKey(model, "provider")
			if len(model.Content) == 0 {
				deleteMapKey(root, "model")
			}
			changed = true
		}
	}

	if !changed {
		_ = os.Remove(h.statePath())
		return nil, nil
	}
	out, err := marshalYAML(doc)
	if err != nil {
		return nil, err
	}
	out = preserveLeadingComments(raw, out)
	backup, err := backupBeforeWriteStrict("hermes", h.configPath())
	if err != nil {
		return nil, err
	}
	if err := atomicWrite(h.configPath(), out); err != nil {
		return nil, err
	}
	_ = os.Remove(h.statePath())
	return []FileChange{{File: h.configPath(), Backup: backup}}, nil
}
