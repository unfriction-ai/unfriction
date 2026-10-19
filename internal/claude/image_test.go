package claude

import (
	"encoding/json"
	"github.com/unfriction-ai/unfriction/internal/run"
	"os"
	"strings"
	"testing"
)

func TestRecordedImageObservations(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		b, err := os.ReadFile("../../fixtures/tool-variants/claude-image-" + format + ".jsonl")
		if err != nil {
			t.Fatal(err)
		}
		r, err := Parse(strings.NewReader(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Events) != 2 || r.Events[1].Kind != run.Observation || r.Events[1].AcquisitionID != r.Events[0].AcquisitionID || r.Events[1].Image == nil || r.Events[1].Image.MediaType != "image/"+format || r.Events[1].Text != "" || len(r.MissingObservations()) != 0 {
			t.Fatalf("image lost or invented text: %+v", r)
		}
	}
}

func TestInvalidImageResults(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/tool-variants/claude-image-png.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{"invalid base64", func(s map[string]interface{}) { s["data"] = "not base64" }},
		{"empty image", func(s map[string]interface{}) { s["data"] = "" }},
		{"not image bytes", func(s map[string]interface{}) { s["data"] = "aGVsbG8=" }},
		{"wrong media type", func(s map[string]interface{}) { s["media_type"] = "image/jpeg" }},
		{"unsupported media type", func(s map[string]interface{}) { s["media_type"] = "image/gif" }},
		{"URL source", func(s map[string]interface{}) { s["type"] = "url" }},
		{"unknown source field", func(s map[string]interface{}) { s["future"] = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := strings.Split(strings.TrimSpace(string(b)), "\n")
			var e map[string]interface{}
			if err := json.Unmarshal([]byte(lines[2]), &e); err != nil {
				t.Fatal(err)
			}
			block := e["message"].(map[string]interface{})["content"].([]interface{})[0].(map[string]interface{})
			item := block["content"].([]interface{})[0].(map[string]interface{})
			tc.mutate(item["source"].(map[string]interface{}))
			modified, _ := json.Marshal(e)
			lines[2] = string(modified)
			if _, err := Parse(strings.NewReader(strings.Join(lines, "\n"))); err == nil || !strings.Contains(err.Error(), "line 3") {
				t.Fatalf("accepted invalid image or lost provenance: %v", err)
			}
		})
	}
}

func TestImageResultsNeverDiscardExtraBlocks(t *testing.T) {
	b, err := os.ReadFile("../../fixtures/tool-variants/claude-image-png.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var e map[string]interface{}
	if err := json.Unmarshal([]byte(lines[2]), &e); err != nil {
		t.Fatal(err)
	}
	block := e["message"].(map[string]interface{})["content"].([]interface{})[0].(map[string]interface{})
	items := block["content"].([]interface{})
	for _, extra := range []interface{}{items[0], map[string]interface{}{"type": "text", "text": "extra"}} {
		block["content"] = []interface{}{items[0], extra}
		modified, _ := json.Marshal(e)
		lines[2] = string(modified)
		if _, err := Parse(strings.NewReader(strings.Join(lines, "\n"))); err == nil {
			t.Fatal("silently dropped extra image result content")
		}
	}
}
