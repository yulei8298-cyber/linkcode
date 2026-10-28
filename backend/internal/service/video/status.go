package video

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// MapStatus 按协议给出的映射表把原生状态转换为统一状态；未知状态视为运行中，
// 让轮询继续而不是误判终态。
func MapStatus(raw string, mapping map[string]Status) Status {
	if s, ok := mapping[strings.ToLower(strings.TrimSpace(raw))]; ok {
		return s
	}
	return StatusRunning
}

// SetBearer 写入 Bearer 鉴权头。
func SetBearer(h http.Header, cred Credentials) error {
	key := strings.TrimSpace(cred.APIKey)
	if key == "" {
		return fmt.Errorf("video account is missing api_key")
	}
	h.Set("Authorization", "Bearer "+key)
	return nil
}

// RequireJSONObject 校验原生请求体是 JSON 对象。
func RequireJSONObject(body []byte) (gjson.Result, error) {
	if !gjson.ValidBytes(body) {
		return gjson.Result{}, invalidf("request body must be valid JSON")
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return gjson.Result{}, invalidf("request body must be a JSON object")
	}
	return root, nil
}

// RequireTaskID 校验并返回非空任务 ID。
func RequireTaskID(v gjson.Result, protocol Protocol) (string, error) {
	id := strings.TrimSpace(v.String())
	if id == "" {
		return "", fmt.Errorf("%s create response is missing task id", protocol)
	}
	return id, nil
}

// SafeSegment 校验拼进上游路径的任务 ID，阻止路径穿越与查询串注入。
func SafeSegment(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && !strings.ContainsAny(id, "/?#\\ ") && id != "." && id != ".."
}

// HasSuffixPath 报告路径是否以给定的段序列结尾（忽略首尾斜杠与版本前缀差异）。
func HasSuffixPath(path string, suffix ...string) bool {
	segs := PathSegments(path)
	if len(segs) < len(suffix) {
		return false
	}
	tail := segs[len(segs)-len(suffix):]
	for i := range suffix {
		if tail[i] != suffix[i] {
			return false
		}
	}
	return true
}

// AfterSegments 在路径中查找连续段 marker，返回其后的剩余段。
func AfterSegments(path string, marker ...string) ([]string, bool) {
	segs := PathSegments(path)
	for i := 0; i+len(marker) <= len(segs); i++ {
		match := true
		for j := range marker {
			if segs[i+j] != marker[j] {
				match = false
				break
			}
		}
		if match {
			return segs[i+len(marker):], true
		}
	}
	return nil, false
}

// RewriteContentURLs 把原生响应中协议声明的成品地址字段替换为 link；
// 协议未声明、字段不存在或不是非空字符串时原样返回。
func RewriteContentURLs(p Provider, body []byte, link string) []byte {
	rewriter, ok := p.(ContentURLRewriter)
	if !ok || link == "" || !gjson.ValidBytes(body) {
		return body
	}
	for _, path := range rewriter.ContentURLPaths() {
		if v := gjson.GetBytes(body, path); v.Type == gjson.String && v.String() != "" {
			if out, err := sjson.SetBytes(body, path, link); err == nil {
				body = out
			}
		}
	}
	return body
}
