package service

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/NursultanKoshoev11/MobileChatServer/internal/media"
)

// stripInlinePublicRequestPhotoMetadata removes EXIF/GPS and text metadata
// from base64 photos embedded in a structured public request body. Bodies that
// are not structured payloads, or that contain no inline photo whose metadata
// changes, are returned unchanged. Unknown fields are preserved.
func stripInlinePublicRequestPhotoMetadata(body string) string {
	raw := strings.TrimSpace(body)
	if !strings.HasPrefix(raw, "{") || !strings.Contains(raw, `"photos"`) {
		return body
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return body
	}
	var photos []map[string]json.RawMessage
	if err := json.Unmarshal(payload["photos"], &photos); err != nil || len(photos) == 0 {
		return body
	}
	changed := false
	for _, photo := range photos {
		var data string
		if err := json.Unmarshal(photo["base64"], &data); err != nil || strings.TrimSpace(data) == "" {
			continue
		}
		data = strings.TrimSpace(data)
		prefix := ""
		if comma := strings.LastIndex(data, ","); comma >= 0 {
			prefix, data = data[:comma+1], data[comma+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil || len(decoded) == 0 {
			continue
		}
		cleaned := media.StripImageMetadata(decoded)
		if len(cleaned) == len(decoded) && string(cleaned) == string(decoded) {
			continue
		}
		encoded, err := json.Marshal(prefix + base64.StdEncoding.EncodeToString(cleaned))
		if err != nil {
			continue
		}
		photo["base64"] = encoded
		if _, ok := photo["size_bytes"]; ok {
			photo["size_bytes"] = json.RawMessage(strconv.Itoa(len(cleaned)))
		}
		changed = true
	}
	if !changed {
		return body
	}
	encodedPhotos, err := json.Marshal(photos)
	if err != nil {
		return body
	}
	payload["photos"] = encodedPhotos
	out, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return string(out)
}
