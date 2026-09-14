package service

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// intelCheckStreamResult 是完整消费一次 SSE 后得到的文本与用量。
type intelCheckStreamResult struct {
	Text         string
	InputTokens  *int
	OutputTokens *int
}

type intelCheckSSEFrame struct {
	Event string
	Data  string
}

// parseIntelCheckUpstreamStream 按上游协议解析完整 SSE 响应。
// 未出现协议终止事件时返回错误，禁止调用方拿半截输出继续判题。
func parseIntelCheckUpstreamStream(apiMode string, body []byte) (intelCheckStreamResult, error) {
	frames := intelCheckSSEDataFrames(body)
	if len(frames) == 0 {
		return intelCheckStreamResult{}, fmt.Errorf("上游流式响应不含 data 事件：%s", intelCheckBodyPreview(body))
	}
	if normalizeIntelCheckAPIMode(apiMode) == IntelCheckAPIModeChatCompletions {
		return parseIntelCheckChatStream(frames)
	}
	return parseIntelCheckResponsesStream(frames)
}

func parseIntelCheckResponsesStream(frames []intelCheckSSEFrame) (intelCheckStreamResult, error) {
	var (
		text              strings.Builder
		doneText          string
		completedResponse string
		completed         bool
		inputTokens       *int
		outputTokens      *int
	)

	for _, frame := range frames {
		data := frame.Data
		if data == "[DONE]" {
			continue
		}
		if !gjson.Valid(data) {
			return intelCheckStreamResult{}, fmt.Errorf("Responses 流包含非法 JSON：%s", intelCheckBodyPreview([]byte(data)))
		}

		eventType := strings.TrimSpace(gjson.Get(data, "type").String())
		if eventType == "" {
			eventType = frame.Event
		}
		if usage, ok := extractOpenAIUsageFromJSONBytes([]byte(data)); ok {
			input, output := usage.InputTokens, usage.OutputTokens
			inputTokens, outputTokens = &input, &output
		}
		switch eventType {
		case "response.output_text.delta":
			text.WriteString(gjson.Get(data, "delta").String())
		case "response.output_text.done":
			doneText = gjson.Get(data, "text").String()
		case "response.completed", "response.done":
			completed = true
			completedResponse = gjson.Get(data, "response").Raw
		case "error", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
			return intelCheckStreamResult{}, fmt.Errorf("Responses 流异常终止（%s）：%s",
				eventType, intelCheckStreamErrorMessage(data))
		}
	}

	if !completed {
		return intelCheckStreamResult{}, fmt.Errorf("Responses 流在完整终止事件前结束")
	}
	output := text.String()
	if strings.TrimSpace(output) == "" {
		output = doneText
	}
	if strings.TrimSpace(output) == "" && completedResponse != "" {
		output = extractOpenAIResponsesText([]byte(completedResponse))
	}
	if strings.TrimSpace(output) == "" {
		return intelCheckStreamResult{}, fmt.Errorf("Responses 流已结束但没有可用的文本内容")
	}
	return intelCheckStreamResult{
		Text: output, InputTokens: inputTokens, OutputTokens: outputTokens,
	}, nil
}

func parseIntelCheckChatStream(frames []intelCheckSSEFrame) (intelCheckStreamResult, error) {
	var (
		text         strings.Builder
		done         bool
		inputTokens  *int
		outputTokens *int
	)

	for _, frame := range frames {
		data := frame.Data
		if data == "[DONE]" {
			done = true
			continue
		}
		if !gjson.Valid(data) {
			return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流包含非法 JSON：%s",
				intelCheckBodyPreview([]byte(data)))
		}
		if frame.Event == "error" || gjson.Get(data, "error").Exists() {
			return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流异常终止：%s",
				intelCheckStreamErrorMessage(data))
		}
		if usage, ok := extractOpenAIUsageFromJSONBytes([]byte(data)); ok {
			input, output := usage.InputTokens, usage.OutputTokens
			inputTokens, outputTokens = &input, &output
		}
		content := gjson.Get(data, "choices.0.delta.content")
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				text.WriteString(block.Get("text").String())
				return true
			})
		} else {
			text.WriteString(content.String())
		}
	}

	if !done {
		return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流在 [DONE] 前结束")
	}
	if strings.TrimSpace(text.String()) == "" {
		return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流已结束但没有可用的文本内容")
	}
	return intelCheckStreamResult{
		Text: text.String(), InputTokens: inputTokens, OutputTokens: outputTokens,
	}, nil
}

// intelCheckSSEDataFrames 返回每个 SSE 事件中所有 data 行的拼接结果。
// SSE 允许一个事件由多行 data 组成，不能只读取第一行。
func intelCheckSSEDataFrames(body []byte) []intelCheckSSEFrame {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	blocks := strings.Split(text, "\n\n")
	frames := make([]intelCheckSSEFrame, 0, len(blocks))
	for _, block := range blocks {
		var (
			event     string
			dataLines []string
		)
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event:"):
				event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		if data := strings.TrimSpace(strings.Join(dataLines, "\n")); data != "" {
			frames = append(frames, intelCheckSSEFrame{Event: event, Data: data})
		}
	}
	return frames
}

func intelCheckStreamErrorMessage(data string) string {
	for _, path := range []string{"error.message", "response.error.message", "message"} {
		if message := strings.TrimSpace(gjson.Get(data, path).String()); message != "" {
			return intelCheckBodyPreview([]byte(message))
		}
	}
	return intelCheckBodyPreview([]byte(data))
}
