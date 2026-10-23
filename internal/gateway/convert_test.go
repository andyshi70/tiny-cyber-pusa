package gateway

import (
	"encoding/json"
	"testing"

	"zen-gate/internal/lane"
)

func TestPartsOfContentAudioAndFile(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"text","text":"听一下"},
		{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}},
		{"type":"input_audio","input_audio":{"data":"QUJD","format":"mp3"}},
		{"type":"input_audio","input_audio":{"data":"QUJD"}},
		{"type":"file","file":{"filename":"doc.pdf","file_data":"data:application/pdf;base64,RkFLRQ=="}}
	]`)
	parts := partsOfContent(raw, nil)
	if len(parts) != 5 {
		t.Fatalf("parts = %d, want 5: %#v", len(parts), parts)
	}
	audio, ok := parts[2].(lane.AudioPart)
	if !ok || audio.Data != "QUJD" || audio.Format != "mp3" {
		t.Fatalf("part 2 = %#v", parts[2])
	}
	if audio2, ok := parts[3].(lane.AudioPart); !ok || audio2.Format != "wav" {
		t.Fatalf("bare input_audio must default to wav: %#v", parts[3])
	}
	file, ok := parts[4].(lane.FilePart)
	if !ok || file.Name != "doc.pdf" || file.MediaType != "application/pdf" || file.Data != "RkFLRQ==" {
		t.Fatalf("part 4 = %#v", parts[4])
	}
}

func TestResponsesContentPartsAudioAndFile(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"input_text","text":"hi"},
		{"type":"input_image","image_url":"data:image/png;base64,AAA"},
		{"type":"input_audio","data":"QUJD","format":"wav"},
		{"type":"input_file","filename":"doc.pdf","file_data":"data:application/pdf;base64,RkFLRQ=="}
	]`)
	parts := responsesContentParts(raw)
	if len(parts) != 4 {
		t.Fatalf("parts = %d, want 4: %#v", len(parts), parts)
	}
	if _, ok := parts[2].(lane.AudioPart); !ok {
		t.Fatalf("part 2 = %#v", parts[2])
	}
	file, ok := parts[3].(lane.FilePart)
	if !ok || file.Name != "doc.pdf" || file.MediaType != "application/pdf" {
		t.Fatalf("part 3 = %#v", parts[3])
	}
}

func TestAnthropicPartsDocument(t *testing.T) {
	raw := json.RawMessage(`[
		{"type":"text","text":"总结"},
		{"type":"document","title":"doc.pdf","source":{"type":"base64","media_type":"application/pdf","data":"RkFLRQ=="}},
		{"type":"image","source":{"type":"base64","media_type":"image/png","data":"AAA"}}
	]`)
	parts := anthropicParts(raw, nil)
	if len(parts) != 3 {
		t.Fatalf("parts = %d, want 3: %#v", len(parts), parts)
	}
	doc, ok := parts[1].(lane.FilePart)
	if !ok || doc.Name != "doc.pdf" || doc.MediaType != "application/pdf" || doc.Data != "RkFLRQ==" {
		t.Fatalf("part 1 = %#v", parts[1])
	}
	if _, ok := parts[2].(lane.ImagePart); !ok {
		t.Fatalf("part 2 = %#v", parts[2])
	}
}

func TestConvertOpenAIMessagesAccumulatesNeeds(t *testing.T) {
	body := []openaiMessage{{
		Role: "user",
		Content: json.RawMessage(`[
			{"type":"text","text":"describe"},
			{"type":"input_audio","input_audio":{"data":"QUJD","format":"wav"}},
			{"type":"file","file":{"file_data":"data:application/pdf;base64,RkFLRQ=="}}
		]`),
	}}
	msgs := convertOpenAIMessages(body)
	needs := lane.SummarizeNeeds(msgs, 1)
	if !needs.Audio || !needs.File || !needs.Tools || needs.Image {
		t.Fatalf("needs = %+v", needs)
	}
}
