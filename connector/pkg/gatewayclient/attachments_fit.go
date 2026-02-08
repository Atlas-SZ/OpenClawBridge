package gatewayclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"strings"
)

// OpenClaw Gateway uses ws maxPayload = 512KB. Keep some headroom for JSON envelope/fields.
const (
	gatewayFrameLimitBytes  = 512 * 1024
	gatewayFrameHeadroom    = 32 * 1024
	gatewayTargetFrameBytes = gatewayFrameLimitBytes - gatewayFrameHeadroom
	minAttachmentBase64Len  = 12 * 1024
)

var (
	jpegQualities = []int{82, 72, 62, 52, 44, 36, 30}
	maxDimensions = []int{1920, 1600, 1280, 1024, 800, 640, 512, 384, 256}
)

func fitGatewayAttachments(method, reqID string, baseParams map[string]any, attachments []map[string]any) ([]map[string]any, bool, int, int, error) {
	if len(attachments) == 0 {
		return attachments, false, 0, 0, nil
	}

	cloned := cloneAttachments(attachments)
	before := estimateGatewayRequestBytes(method, reqID, withAttachments(baseParams, cloned))
	if before <= gatewayTargetFrameBytes {
		return cloned, false, before, before, nil
	}

	baseNoAttachments := cloneMap(baseParams)
	delete(baseNoAttachments, "attachments")
	baseOnlyBytes := estimateGatewayRequestBytes(method, reqID, baseNoAttachments)
	if baseOnlyBytes >= gatewayTargetFrameBytes {
		return nil, false, before, before, fmt.Errorf("gateway payload too large before attachments (%d bytes)", baseOnlyBytes)
	}

	perAttachmentTarget := (gatewayTargetFrameBytes - baseOnlyBytes) / len(cloned)
	if perAttachmentTarget < minAttachmentBase64Len {
		perAttachmentTarget = minAttachmentBase64Len
	}

	changed := false
	for i := range cloned {
		compressed, didChange, err := tryCompressAttachment(cloned[i], perAttachmentTarget)
		if err != nil {
			return nil, false, before, before, err
		}
		cloned[i] = compressed
		changed = changed || didChange
	}

	after := estimateGatewayRequestBytes(method, reqID, withAttachments(baseParams, cloned))
	for step := 0; after > gatewayTargetFrameBytes && step < 8; step++ {
		idx := largestAttachmentIndex(cloned)
		if idx < 0 {
			break
		}
		curLen := attachmentContentLen(cloned[idx])
		if curLen <= minAttachmentBase64Len {
			break
		}
		nextTarget := int(float64(curLen) * 0.75)
		if nextTarget < minAttachmentBase64Len {
			nextTarget = minAttachmentBase64Len
		}
		next, didChange, err := tryCompressAttachment(cloned[idx], nextTarget)
		if err != nil {
			return nil, false, before, after, err
		}
		if !didChange || attachmentContentLen(next) >= curLen {
			break
		}
		cloned[idx] = next
		changed = true
		after = estimateGatewayRequestBytes(method, reqID, withAttachments(baseParams, cloned))
	}

	if after > gatewayTargetFrameBytes {
		return nil, false, before, after, fmt.Errorf("attachment payload still too large for gateway after compression (%d bytes > %d bytes). use media URL workflow for large files", after, gatewayTargetFrameBytes)
	}
	return cloned, changed, before, after, nil
}

func tryCompressAttachment(att map[string]any, targetBase64Len int) (map[string]any, bool, error) {
	content, ok := stringFromAny(att["content"])
	if !ok {
		return att, false, nil
	}
	content = strings.TrimSpace(content)
	if content == "" || len(content) <= targetBase64Len {
		return att, false, nil
	}

	mime, _ := stringFromAny(att["mimeType"])
	if !isImageMime(mime) {
		name := attachmentName(att)
		return nil, false, fmt.Errorf("%s is too large and cannot be auto-compressed (mime=%q). use media URL workflow", name, strings.TrimSpace(mime))
	}

	decoded, err := decodeAttachmentBase64(content)
	if err != nil {
		return nil, false, fmt.Errorf("%s has invalid base64 content: %w", attachmentName(att), err)
	}

	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		return nil, false, fmt.Errorf("%s cannot be decoded as image: %w", attachmentName(att), err)
	}

	best := []byte(nil)
	bestLen := 0
	for _, maxDim := range maxDimensions {
		resized := resizeDownIfNeeded(img, maxDim)
		for _, q := range jpegQualities {
			buf := new(bytes.Buffer)
			if err := jpeg.Encode(buf, resized, &jpeg.Options{Quality: q}); err != nil {
				continue
			}
			candidate := base64.StdEncoding.EncodeToString(buf.Bytes())
			if best == nil || len(candidate) < bestLen {
				best = []byte(candidate)
				bestLen = len(candidate)
			}
			if len(candidate) <= targetBase64Len {
				out := cloneAttachment(att)
				out["content"] = candidate
				out["mimeType"] = "image/jpeg"
				out["type"] = "image"
				if fn, ok := stringFromAny(out["fileName"]); ok && fn != "" {
					out["fileName"] = replaceExtension(fn, ".jpg")
				}
				return out, true, nil
			}
		}
	}

	if best != nil {
		out := cloneAttachment(att)
		out["content"] = string(best)
		out["mimeType"] = "image/jpeg"
		out["type"] = "image"
		if fn, ok := stringFromAny(out["fileName"]); ok && fn != "" {
			out["fileName"] = replaceExtension(fn, ".jpg")
		}
		return out, true, nil
	}
	return att, false, nil
}

func estimateGatewayRequestBytes(method, reqID string, params map[string]any) int {
	msg := map[string]any{
		"type":   "req",
		"id":     reqID,
		"method": method,
		"params": params,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return 0
	}
	return len(b)
}

func withAttachments(base map[string]any, attachments []map[string]any) map[string]any {
	out := cloneMap(base)
	if len(attachments) > 0 {
		out["attachments"] = attachments
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneAttachment(att map[string]any) map[string]any {
	out := make(map[string]any, len(att))
	for k, v := range att {
		out[k] = v
	}
	return out
}

func cloneAttachments(in []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, att := range in {
		out = append(out, cloneAttachment(att))
	}
	return out
}

func largestAttachmentIndex(attachments []map[string]any) int {
	maxLen := 0
	idx := -1
	for i := range attachments {
		n := attachmentContentLen(attachments[i])
		if n > maxLen {
			maxLen = n
			idx = i
		}
	}
	return idx
}

func attachmentContentLen(att map[string]any) int {
	content, ok := stringFromAny(att["content"])
	if !ok {
		return 0
	}
	return len(strings.TrimSpace(content))
}

func attachmentName(att map[string]any) string {
	if name, ok := stringFromAny(att["fileName"]); ok && strings.TrimSpace(name) != "" {
		return fmt.Sprintf("attachment %q", strings.TrimSpace(name))
	}
	return "attachment"
}

func stringFromAny(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}

func isImageMime(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))
	return strings.HasPrefix(m, "image/") || m == ""
}

func decodeAttachmentBase64(content string) ([]byte, error) {
	s := strings.TrimSpace(content)
	if i := strings.IndexByte(s, ','); i > 0 && strings.Contains(strings.ToLower(s[:i]), "base64") {
		s = s[i+1:]
	}
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\n', '\r', '\t':
			return -1
		default:
			return r
		}
	}, s)
	return base64.StdEncoding.DecodeString(s)
}

func replaceExtension(name, newExt string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if idx := strings.LastIndexByte(name, '.'); idx > 0 {
		return name[:idx] + newExt
	}
	return name + newExt
}

func resizeDownIfNeeded(src image.Image, maxDim int) image.Image {
	if maxDim <= 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}
	if w <= 0 || h <= 0 {
		return src
	}

	scale := float64(maxDim) / float64(w)
	if h > w {
		scale = float64(maxDim) / float64(h)
	}
	dstW := int(float64(w) * scale)
	dstH := int(float64(h) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := b.Min.Y + (y*h)/dstH
		for x := 0; x < dstW; x++ {
			srcX := b.Min.X + (x*w)/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	// Fill transparent pixels with white before JPEG encode.
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			r, g, bCh, a := dst.At(x, y).RGBA()
			if a == 0 {
				dst.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
				continue
			}
			if a < 0xffff {
				alpha := float64(a) / 65535.0
				rr := uint8((float64(r>>8) * alpha) + (255.0 * (1.0 - alpha)))
				gg := uint8((float64(g>>8) * alpha) + (255.0 * (1.0 - alpha)))
				bb := uint8((float64(bCh>>8) * alpha) + (255.0 * (1.0 - alpha)))
				dst.Set(x, y, color.RGBA{R: rr, G: gg, B: bb, A: 255})
			}
		}
	}

	return dst
}
