package lane

import "testing"

func TestNameCapabilitiesPopularFamilies(t *testing.T) {
	cases := []struct {
		id            string
		vision, audio bool
		file          bool
		reasoning     bool
	}{
		{"gpt-4o-mini", true, false, true, false},
		{"gpt-4.1-nano", true, false, true, false},
		{"gemini-2.5-flash", true, true, true, false},
		{"claude-sonnet-4-5", true, false, true, false},
		{"qwen2.5-vl-72b", true, false, true, false},
		{"qwen-omni-turbo", true, true, false, false},
		{"deepseek-r1-distill", false, false, false, true},
		{"o4-mini", true, false, false, true},
		{"mistral-small-3.1", true, false, true, false},
	}
	for _, c := range cases {
		got := NameCapabilities(c.id)
		if !got.Matched {
			t.Fatalf("%s: expected a match", c.id)
		}
		if got.Vision != c.vision || got.Audio != c.audio || got.File != c.file || got.Reasoning != c.reasoning {
			t.Fatalf("%s: got %+v, want vision=%v audio=%v file=%v reasoning=%v",
				c.id, got, c.vision, c.audio, c.file, c.reasoning)
		}
	}
}

func TestNameCapabilitiesUnknownStaysUnmatched(t *testing.T) {
	for _, id := range []string{"totally-unknown-model", "my-private-llm-v9", ""} {
		if got := NameCapabilities(id); got.Matched {
			t.Fatalf("%q: unexpected match %+v", id, got)
		}
	}
}
