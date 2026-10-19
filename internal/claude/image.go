package claude

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/unfriction-ai/unfriction/internal/run"
)

// imageObservation accepts the recorded single-image result. It checks encoding
// and image headers without interpreting pixels or retaining image bytes.
func imageObservation(raw json.RawMessage) (run.ImageObservation, error) {
	var items []struct {
		Type   string `json:"type"`
		Source struct {
			Type      string `json:"type"`
			Data      string `json:"data"`
			MediaType string `json:"media_type"`
		} `json:"source"`
	}
	fail := func() (run.ImageObservation, error) {
		return run.ImageObservation{}, fmt.Errorf("observation must be text or one base64 PNG/JPEG image with matching media type")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&items); err != nil || len(items) != 1 {
		return fail()
	}
	item := items[0]
	if item.Type != "image" || item.Source.Type != "base64" || (item.Source.MediaType != "image/png" && item.Source.MediaType != "image/jpeg") {
		return fail()
	}
	data, err := base64.StdEncoding.Strict().DecodeString(item.Source.Data)
	if err != nil || len(data) == 0 {
		return fail()
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || "image/"+format != item.Source.MediaType {
		return fail()
	}
	return run.ImageObservation{MediaType: item.Source.MediaType, ByteLength: len(data)}, nil
}
