package lane

import "testing"

func TestChatWireCarriesAudioAndFile(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Parts: []Part{
		TextPart{Text: "listen"},
		AudioPart{Data: "QUJD", Format: "mp3"},
		FilePart{Name: "doc.pdf", MediaType: "application/pdf", Data: "RkFLRQ=="},
	}}}
	out := ToChatMessages(msgs)
	if len(out) != 2 { // text turn, then the parts turn
		t.Fatalf("turns = %d, want 2: %+v", len(out), out)
	}
	parts, ok := out[1]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("parts turn content = %#v", out[1]["content"])
	}
	if parts[0]["type"] != "input_audio" {
		t.Fatalf("part 0 = %#v", parts[0])
	}
	audio := parts[0]["input_audio"].(map[string]any)
	if audio["data"] != "QUJD" || audio["format"] != "mp3" {
		t.Fatalf("audio = %#v", audio)
	}
	if parts[1]["type"] != "file" {
		t.Fatalf("part 1 = %#v", parts[1])
	}
	file := parts[1]["file"].(map[string]any)
	if file["filename"] != "doc.pdf" || file["file_data"] != "data:application/pdf;base64,RkFLRQ==" {
		t.Fatalf("file = %#v", file)
	}
}

func TestClaudeWireTakesPDFDropsAudio(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Parts: []Part{
		AudioPart{Data: "QUJD", Format: "wav"}, // Anthropic has no audio input
		FilePart{Name: "doc.pdf", MediaType: "application/pdf", Data: "RkFLRQ=="},
		FilePart{Name: "nope.bin", MediaType: "application/octet-stream", Data: "RkFLRQ=="},
	}}}
	_, out := ToClaudeMessages(msgs)
	if len(out) != 1 {
		t.Fatalf("turns = %d", len(out))
	}
	blocks := out[0]["content"].([]map[string]any)
	if len(blocks) != 1 || blocks[0]["type"] != "document" {
		t.Fatalf("blocks = %#v", blocks)
	}
	src := blocks[0]["source"].(map[string]any)
	if src["media_type"] != "application/pdf" || src["data"] != "RkFLRQ==" {
		t.Fatalf("source = %#v", src)
	}
}

func TestResponsesWireCarriesAudioAndFile(t *testing.T) {
	msgs := []Message{{Role: RoleUser, Parts: []Part{
		AudioPart{Data: "QUJD", Format: "wav"},
		FilePart{Name: "doc.pdf", MediaType: "application/pdf", Data: "RkFLRQ=="},
	}}}
	_, out := ToResponseInput(msgs)
	if len(out) != 1 {
		t.Fatalf("turns = %d", len(out))
	}
	items := out[0]["content"].([]map[string]any)
	if items[0]["type"] != "input_audio" || items[0]["format"] != "wav" {
		t.Fatalf("item 0 = %#v", items[0])
	}
	if items[1]["type"] != "input_file" || items[1]["filename"] != "doc.pdf" ||
		items[1]["file_data"] != "data:application/pdf;base64,RkFLRQ==" {
		t.Fatalf("item 1 = %#v", items[1])
	}
}

func TestRepairToolPairingKeepsNewParts(t *testing.T) {
	msgs := []Message{
		{Role: RoleAssistant, Parts: []Part{ToolCallPart{ID: "t1", Name: "f"}}},
		{Role: RoleUser, Parts: []Part{ImagePart{DataURL: "data:image/png;base64,AA"}, AudioPart{Data: "QQ", Format: "wav"}}},
	}
	out := RepairToolPairing(msgs)
	if len(out) != 1 {
		t.Fatalf("turns = %d, want 1 (dangling tool call dropped)", len(out))
	}
	if len(out[0].Parts) != 2 {
		t.Fatalf("parts = %d, want 2 (audio/file untouched by the repair)", len(out[0].Parts))
	}
}
