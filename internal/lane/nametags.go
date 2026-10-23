package lane

import "regexp"

// NameTags is the offline capability guess derived from a model id alone.
// Matched reports whether any pattern recognized the id — an unmatched id
// stays "unknown" and the router ranks it after known-capable candidates
// instead of trusting a guess.
type NameTags struct {
	Vision        bool
	Audio         bool
	File          bool
	Reasoning     bool
	ContextWindow int
	MaxOutput     int
	Matched       bool
}

// Heuristic tables for the long tail of user-added provider models. They only
// need to be right about the popular families — the AI tagger refines or
// overrides them, and a live capability probe overrides both.
var (
	visionRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^gpt-4\.?o`),
		regexp.MustCompile(`(?i)^gpt-4\.?1`),
		regexp.MustCompile(`(?i)^gpt-5`),
		regexp.MustCompile(`(?i)^chatgpt-4o`),
		regexp.MustCompile(`(?i)^o[34](-|$|@)`),
		regexp.MustCompile(`(?i)gemini`),
		regexp.MustCompile(`(?i)^claude-(3|4|sonnet|opus|haiku)`),
		regexp.MustCompile(`(?i)grok-[34]`),
		regexp.MustCompile(`(?i)(^|[-_])vl([-_.]|$)|vision`),
		regexp.MustCompile(`(?i)^qwen.*(-vl|omni)`),
		regexp.MustCompile(`(?i)^glm-4v|^glm-4\.[56]|^glm-5`),
		regexp.MustCompile(`(?i)pixtral`),
		regexp.MustCompile(`(?i)mistral-(small|medium|large)-3|^mistral-(small|medium|large)`),
		regexp.MustCompile(`(?i)internvl|deepseek-vl|minicpm-v`),
		regexp.MustCompile(`(?i)doubao.*vision|seed-1\.[56]`),
		regexp.MustCompile(`(?i)step-1v|step-3`),
		regexp.MustCompile(`(?i)kimi-|^moonshot`),
		regexp.MustCompile(`(?i)llama-(3\.2|4).*(11b|90b|vision|maverick|scout)`),
		regexp.MustCompile(`(?i)ernie-4\.5|hunyuan-(vision|t1|turbo)|^ernie`),
		regexp.MustCompile(`(?i)phi-3\.5-vision|phi-4`),
		regexp.MustCompile(`(?i)lfm-?7b|^lfm`),
	}
	audioRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)gpt-4o-audio|gpt-audio|chatgpt-4o-audio`),
		regexp.MustCompile(`(?i)qwen.*(-|_)?(audio|omni)`),
		regexp.MustCompile(`(?i)gemini`), // Gemini accepts audio on every current generation
		regexp.MustCompile(`(?i)step-audio|kimi-audio|glm-4-voice`),
		regexp.MustCompile(`(?i)moshi|^voxtral`),
	}
	fileRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^claude-`), // every Claude 3+ takes PDF documents
		regexp.MustCompile(`(?i)gemini`),
		regexp.MustCompile(`(?i)^gpt-4\.?[o1]|^gpt-5|chatgpt-4o`),
		regexp.MustCompile(`(?i)qwen-long|qwen2\.5`),
		regexp.MustCompile(`(?i)mistral-(small|medium|large|nemo)`),
		regexp.MustCompile(`(?i)doubao|^ernie|hunyuan`),
	}
	reasoningRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^o[134](-|$|@)`),
		regexp.MustCompile(`(?i)deepseek-r1|deepseek-reasoner`),
		regexp.MustCompile(`(?i)qwq|^qvq`),
		regexp.MustCompile(`(?i)thinking|reasoning|^r1`),
		regexp.MustCompile(`(?i)glm-z1|gpto1|magistral`),
	}
)

func matchAny(res []*regexp.Regexp, id string) bool {
	for _, re := range res {
		if re.MatchString(id) {
			return true
		}
	}
	return false
}

// NameCapabilities guesses a model's modalities from its id. Used for
// user-added provider models the AI tagger has not classified yet; always a
// floor, never a verdict.
func NameCapabilities(modelID string) NameTags {
	t := NameTags{}
	t.Vision = matchAny(visionRes, modelID)
	t.Audio = matchAny(audioRes, modelID)
	t.File = matchAny(fileRes, modelID)
	t.Reasoning = matchAny(reasoningRes, modelID)
	t.Matched = t.Vision || t.Audio || t.File || t.Reasoning
	return t
}
