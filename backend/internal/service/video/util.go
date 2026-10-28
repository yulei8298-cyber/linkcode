package video

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// 归一化分辨率档位，与计费档位一致。
const (
	Resolution480P  = "480p"
	Resolution720P  = "720p"
	Resolution1080P = "1080p"
	Resolution4K    = "4k"
)

// NormalizeResolution 把各种写法（720P、hd、1280x720、SR=720）归一为档位；无法识别返回空。
func NormalizeResolution(raw string) string {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "":
		return ""
	case "480", "480p", "540", "540p", "360p", "sd":
		return Resolution480P
	case "720", "720p", "768p", "hd":
		return Resolution720P
	case "1080", "1080p", "1024p", "full_hd", "full-hd", "fhd":
		return Resolution1080P
	case "2160", "2160p", "4k", "uhd":
		return Resolution4K
	}
	if w, h, ok := ParseSize(v); ok {
		short := min(w, h)
		switch {
		case short >= 2000:
			return Resolution4K
		case short >= 1000:
			return Resolution1080P
		case short >= 700:
			return Resolution720P
		default:
			return Resolution480P
		}
	}
	return ""
}

// ParseSize 解析 1280x720 / 1280*720 / 1280:720 形式的像素尺寸。
func ParseSize(raw string) (int, int, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	for _, sep := range []string{"x", "*", ":"} {
		parts := strings.Split(raw, sep)
		if len(parts) != 2 {
			continue
		}
		w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
		h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
		if errW == nil && errH == nil && w > 0 && h > 0 && (w > 64 || h > 64) {
			return w, h, true
		}
	}
	return 0, 0, false
}

// AspectRatioFromSize 由像素尺寸推出常用画幅（16:9、9:16、1:1、4:3、3:4、21:9）。
func AspectRatioFromSize(size string) string {
	w, h, ok := ParseSize(size)
	if !ok {
		return ""
	}
	ratio := float64(w) / float64(h)
	candidates := []struct {
		label string
		value float64
	}{{"16:9", 16.0 / 9}, {"9:16", 9.0 / 16}, {"1:1", 1}, {"4:3", 4.0 / 3}, {"3:4", 3.0 / 4}, {"21:9", 21.0 / 9}}
	best, bestDiff := "", 1e9
	for _, c := range candidates {
		diff := ratio - c.value
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best, bestDiff = c.label, diff
		}
	}
	return best
}

// SizeFor 由档位与画幅生成像素尺寸（OpenAI /v1/videos 的 size 参数）。
func SizeFor(resolution, aspect string) string {
	short := map[string]int{Resolution480P: 480, Resolution720P: 720, Resolution1080P: 1080, Resolution4K: 2160}[resolution]
	if short == 0 {
		short = 720
	}
	long := short * 16 / 9
	switch aspect {
	case "9:16":
		return fmt.Sprintf("%dx%d", short, long)
	case "1:1":
		return fmt.Sprintf("%dx%d", short, short)
	case "4:3":
		return fmt.Sprintf("%dx%d", short*4/3, short)
	case "3:4":
		return fmt.Sprintf("%dx%d", short, short*4/3)
	default:
		return fmt.Sprintf("%dx%d", long, short)
	}
}

// JoinURL 拼接 Base URL 与协议路径，并去掉两者重叠的版本前缀：Base 写成
// https://host/v1 或 https://host 时拼接 /v1/videos 都得到 https://host/v1/videos。
func JoinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	schemeEnd := strings.Index(base, "://")
	pathStart := -1
	if schemeEnd >= 0 {
		if idx := strings.Index(base[schemeEnd+3:], "/"); idx >= 0 {
			pathStart = schemeEnd + 3 + idx
		}
	}
	if pathStart < 0 {
		return base + path
	}
	baseSegs := strings.Split(strings.Trim(base[pathStart:], "/"), "/")
	pathSegs := strings.Split(strings.Trim(path, "/"), "/")
	for n := min(len(baseSegs), len(pathSegs)); n > 0; n-- {
		if strings.Join(baseSegs[len(baseSegs)-n:], "/") == strings.Join(pathSegs[:n], "/") {
			rest := strings.Join(pathSegs[n:], "/")
			if rest == "" {
				return base
			}
			return base + "/" + rest
		}
	}
	return base + path
}

// BuildBody 把固定字段与 Extra 合并为 JSON 请求体；Extra 中的同名字段覆盖固定字段，
// 便于调用方直接透传厂商新增参数。
func BuildBody(fields map[string]any, extra map[string]json.RawMessage) ([]byte, error) {
	out := make(map[string]any, len(fields)+len(extra))
	for k, v := range fields {
		if !isZeroField(v) {
			out[k] = v
		}
	}
	for k, v := range extra {
		out[k] = v
	}
	return json.Marshal(out)
}

func isZeroField(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case int:
		return t == 0
	case []string:
		return len(t) == 0
	case []map[string]any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case *bool:
		return t == nil
	case *int64:
		return t == nil
	}
	return false
}

// MergeObject 把 extra 合并进已有对象（用于嵌套的 parameters 等字段）。
func MergeObject(dst map[string]any, extra map[string]json.RawMessage, keys ...string) {
	for _, key := range keys {
		raw, ok := extra[key]
		if !ok {
			continue
		}
		var nested map[string]any
		if json.Unmarshal(raw, &nested) == nil {
			for k, v := range nested {
				dst[k] = v
			}
		}
	}
}

// WithoutKeys 返回去掉指定键的 Extra 副本。
func WithoutKeys(extra map[string]json.RawMessage, keys ...string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(extra))
	for k, v := range extra {
		out[k] = v
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

// FormatDurationSeconds 把秒数格式化为字符串，供要求字符串时长的协议使用。
func FormatDurationSeconds(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	return strconv.Itoa(seconds)
}

// PathSegments 返回路径按 / 切分后去掉空段的结果，供协议实现分类原生请求。
func PathSegments(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// BoolPtr 返回布尔指针。
func BoolPtr(v bool) *bool { return &v }
