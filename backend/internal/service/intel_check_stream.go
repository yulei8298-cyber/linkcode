package service

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/tidwall/gjson"
)

const (
	// 单个 SSE 事件允许高于最终文本上限，因为 Responses 的完成事件可能同时
	// 携带 reasoning 与完整 response。限制单个事件而不是整条流，避免无界内存。
	intelCheckStreamEventMaxBytes = 16 << 20
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

type intelCheckStreamConsumer interface {
	Consume(frame intelCheckSSEFrame) (terminal bool, err error)
	Result() (intelCheckStreamResult, error)
}

// readIntelCheckUpstreamStream 逐事件消费 SSE，不缓存整条响应。
// reasoning 等无关事件处理完即可释放，最终只累计模型输出文本与 usage。
func readIntelCheckUpstreamStream(apiMode string, r io.Reader) (intelCheckStreamResult, error) {
	consumer := newIntelCheckStreamConsumer(apiMode)
	parser := openAICompatSSEFrameParser{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), intelCheckStreamEventMaxBytes+1024)

	eventBytes := 0
	sawFrame := false
	consume := func(frame openAICompatSSEFrame) (bool, error) {
		sawFrame = true
		return consumer.Consume(intelCheckSSEFrame{Event: frame.EventType, Data: frame.Data})
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if data, ok := extractOpenAISSEDataLine(line); ok {
			eventBytes += len(data) + 1
			if eventBytes > intelCheckStreamEventMaxBytes {
				return intelCheckStreamResult{}, fmt.Errorf(
					"上游 SSE 单个事件超过 %d 字节上限", intelCheckStreamEventMaxBytes)
			}
		}

		frame, ok := parser.AddLine(line)
		if line == "" {
			eventBytes = 0
		}
		if !ok {
			continue
		}
		terminal, err := consume(frame)
		if err != nil {
			return intelCheckStreamResult{}, err
		}
		if terminal {
			return consumer.Result()
		}
	}
	if err := scanner.Err(); err != nil {
		return intelCheckStreamResult{}, fmt.Errorf(
			"读取上游 SSE 失败（单行上限 %d 字节）：%w", intelCheckStreamEventMaxBytes, err)
	}
	if frame, ok := parser.Finish(); ok {
		terminal, err := consume(frame)
		if err != nil {
			return intelCheckStreamResult{}, err
		}
		if terminal {
			return consumer.Result()
		}
	}
	if !sawFrame {
		return intelCheckStreamResult{}, fmt.Errorf("上游流式响应不含 data 事件")
	}
	return consumer.Result()
}

func newIntelCheckStreamConsumer(apiMode string) intelCheckStreamConsumer {
	if normalizeIntelCheckAPIMode(apiMode) == IntelCheckAPIModeChatCompletions {
		return &intelCheckChatStreamConsumer{}
	}
	return &intelCheckResponsesStreamConsumer{}
}

type intelCheckResponsesStreamConsumer struct {
	text          strings.Builder
	doneText      string
	completedText string
	completed     bool
	inputTokens   *int
	outputTokens  *int
}

func (c *intelCheckResponsesStreamConsumer) Consume(frame intelCheckSSEFrame) (bool, error) {
	data := strings.TrimSpace(frame.Data)
	if data == "[DONE]" {
		return false, nil
	}
	if !gjson.Valid(data) {
		return false, fmt.Errorf("Responses 流包含非法 JSON：%s", intelCheckBodyPreview([]byte(data)))
	}

	eventType := strings.TrimSpace(gjson.Get(data, "type").String())
	if eventType == "" {
		eventType = strings.TrimSpace(frame.Event)
	}
	if usage, ok := extractOpenAIUsageFromJSONBytes([]byte(data)); ok {
		input, output := usage.InputTokens, usage.OutputTokens
		c.inputTokens, c.outputTokens = &input, &output
	}

	switch eventType {
	case "response.output_text.delta":
		if err := appendIntelCheckStreamText(&c.text, gjson.Get(data, "delta").String()); err != nil {
			return false, err
		}
	case "response.output_text.done":
		text := gjson.Get(data, "text").String()
		if err := validateIntelCheckStreamText(text); err != nil {
			return false, err
		}
		c.doneText = text
	case "response.completed", "response.done":
		c.completed = true
		if response := gjson.Get(data, "response"); response.Exists() {
			text := extractOpenAIResponsesText([]byte(response.Raw))
			if err := validateIntelCheckStreamText(text); err != nil {
				return false, err
			}
			c.completedText = text
		}
		return true, nil
	case "error", "response.failed", "response.incomplete", "response.cancelled", "response.canceled":
		return false, fmt.Errorf("Responses 流异常终止（%s）：%s",
			eventType, intelCheckStreamErrorMessage(data))
	}
	return false, nil
}

func (c *intelCheckResponsesStreamConsumer) Result() (intelCheckStreamResult, error) {
	if !c.completed {
		return intelCheckStreamResult{}, fmt.Errorf("Responses 流在完整终止事件前结束")
	}
	output := c.text.String()
	if strings.TrimSpace(output) == "" {
		output = c.doneText
	}
	if strings.TrimSpace(output) == "" {
		output = c.completedText
	}
	if strings.TrimSpace(output) == "" {
		return intelCheckStreamResult{}, fmt.Errorf("Responses 流已结束但没有可用的文本内容")
	}
	return intelCheckStreamResult{
		Text: output, InputTokens: c.inputTokens, OutputTokens: c.outputTokens,
	}, nil
}

type intelCheckChatStreamConsumer struct {
	text         strings.Builder
	done         bool
	inputTokens  *int
	outputTokens *int
}

func (c *intelCheckChatStreamConsumer) Consume(frame intelCheckSSEFrame) (bool, error) {
	data := strings.TrimSpace(frame.Data)
	if data == "[DONE]" {
		c.done = true
		return true, nil
	}
	if !gjson.Valid(data) {
		return false, fmt.Errorf("Chat Completions 流包含非法 JSON：%s",
			intelCheckBodyPreview([]byte(data)))
	}
	if frame.Event == "error" || gjson.Get(data, "error").Exists() {
		return false, fmt.Errorf("Chat Completions 流异常终止：%s",
			intelCheckStreamErrorMessage(data))
	}
	if usage, ok := extractOpenAIUsageFromJSONBytes([]byte(data)); ok {
		input, output := usage.InputTokens, usage.OutputTokens
		c.inputTokens, c.outputTokens = &input, &output
	}

	content := gjson.Get(data, "choices.0.delta.content")
	if content.IsArray() {
		var appendErr error
		content.ForEach(func(_, block gjson.Result) bool {
			appendErr = appendIntelCheckStreamText(&c.text, block.Get("text").String())
			return appendErr == nil
		})
		if appendErr != nil {
			return false, appendErr
		}
	} else if err := appendIntelCheckStreamText(&c.text, content.String()); err != nil {
		return false, err
	}
	return false, nil
}

func (c *intelCheckChatStreamConsumer) Result() (intelCheckStreamResult, error) {
	if !c.done {
		return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流在 [DONE] 前结束")
	}
	if strings.TrimSpace(c.text.String()) == "" {
		return intelCheckStreamResult{}, fmt.Errorf("Chat Completions 流已结束但没有可用的文本内容")
	}
	return intelCheckStreamResult{
		Text: c.text.String(), InputTokens: c.inputTokens, OutputTokens: c.outputTokens,
	}, nil
}

func appendIntelCheckStreamText(dst *strings.Builder, text string) error {
	if len(text) > intelCheckResponseMaxBytes-dst.Len() {
		return fmt.Errorf("上游流式最终文本超过 %d 字节上限", intelCheckResponseMaxBytes)
	}
	dst.WriteString(text)
	return nil
}

func validateIntelCheckStreamText(text string) error {
	if len(text) > intelCheckResponseMaxBytes {
		return fmt.Errorf("上游流式最终文本超过 %d 字节上限", intelCheckResponseMaxBytes)
	}
	return nil
}

func intelCheckStreamErrorMessage(data string) string {
	for _, path := range []string{"error.message", "response.error.message", "message"} {
		if message := strings.TrimSpace(gjson.Get(data, path).String()); message != "" {
			return intelCheckBodyPreview([]byte(message))
		}
	}
	return intelCheckBodyPreview([]byte(data))
}
