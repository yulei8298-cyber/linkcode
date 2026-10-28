// Package video 定义视频生成网关与各上游协议之间的统一模型。
//
// 网关对外暴露统一的任务接口（OpenAI /v1/videos 风格与 new-api /v1/video/generations
// 风格），同时支持各厂商原生协议透传。每个上游协议实现一个 Provider，负责把统一请求
// 转换为原生调用、解析原生响应；网关层只依赖本包的类型，不感知具体厂商。
package video

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

// Protocol 标识上游视频接口协议，存于账号凭据 credentials["video_protocol"]。
type Protocol string

const (
	// ProtocolOpenAIVideos 是 OpenAI /v1/videos（Sora 风格），大量第三方中转沿用。
	ProtocolOpenAIVideos Protocol = "openai_videos"
	// ProtocolNewAPI 是 new-api 统一视频任务接口 /v1/video/generations。
	ProtocolNewAPI Protocol = "newapi"
	// ProtocolArk 是火山方舟 / BytePlus ModelArk（Seedance、豆包视频）。
	ProtocolArk Protocol = "ark"
	// ProtocolKling 是快手可灵开放平台。
	ProtocolKling Protocol = "kling"
	// ProtocolMiniMax 是 MiniMax 海螺视频。
	ProtocolMiniMax Protocol = "minimax"
	// ProtocolDashScope 是阿里云百炼 DashScope（通义万相 Wan）。
	ProtocolDashScope Protocol = "dashscope"
	// ProtocolVeo 是 Google Gemini API 的 Veo（predictLongRunning）。
	ProtocolVeo Protocol = "veo"
	// ProtocolVidu 是生数科技 Vidu。
	ProtocolVidu Protocol = "vidu"
	// ProtocolRunway 是 Runway API。
	ProtocolRunway Protocol = "runway"
	// ProtocolLuma 是 Luma Dream Machine API。
	ProtocolLuma Protocol = "luma"
	// ProtocolPixVerse 是 PixVerse 开放平台。
	ProtocolPixVerse Protocol = "pixverse"
)

// CredentialKeyProtocol 是账号凭据中保存视频协议的键。
const CredentialKeyProtocol = "video_protocol"

// Status 是统一的任务状态。
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "in_progress"
	StatusSucceeded Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// IsTerminal 报告状态是否为终态。
func (s Status) IsTerminal() bool {
	return s == StatusSucceeded || s == StatusFailed || s == StatusCancelled
}

// 图片素材的用途。
const (
	ImageRoleFirstFrame = "first_frame"
	ImageRoleLastFrame  = "last_frame"
	ImageRoleReference  = "reference"
)

// Image 是输入图片素材，URL 可以是 http(s) 地址或 data URI。
type Image struct {
	URL  string
	Role string
}

// Request 是协议无关的视频生成请求。
type Request struct {
	Model           string
	Prompt          string
	NegativePrompt  string
	Images          []Image
	Videos          []string
	Audios          []string
	DurationSeconds int
	// Resolution 为归一化档位（480p / 720p / 1080p / 4k），未指定时为空。
	Resolution  string
	AspectRatio string
	// Size 是客户端原样提交的像素尺寸（如 1280x720），部分协议直接使用。
	Size          string
	GenerateAudio *bool
	Seed          *int64
	// Extra 是厂商专有参数，原样合并进上游请求体。
	Extra map[string]json.RawMessage
}

// FirstImage 返回指定用途的第一张图片地址。
func (r Request) FirstImage(role string) string {
	for _, img := range r.Images {
		if img.Role == role {
			return img.URL
		}
	}
	return ""
}

// ImageURLs 返回全部图片地址（保持提交顺序）。
func (r Request) ImageURLs() []string {
	out := make([]string, 0, len(r.Images))
	for _, img := range r.Images {
		out = append(out, img.URL)
	}
	return out
}

// Call 描述一次上游 HTTP 调用；Path 相对于账号 Base URL。
type Call struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
	// ContentType 为空时按 application/json 发送（有 Body 时）。
	ContentType string
	// Headers 是该调用额外需要的请求头（如 DashScope 的异步开关）。
	Headers http.Header
}

// CreateResult 是上游受理创建请求后返回的任务标识。
type CreateResult struct {
	TaskID string
	// Meta 保存查询时需要的附加信息（如可灵的任务类型、Veo 的模型名）。
	Meta string
}

// Snapshot 是一次查询得到的任务状态。
type Snapshot struct {
	Status   Status
	Progress int
	// VideoURL 是成品的绝对下载地址；ContentPath 非空时改为相对 Base URL 带鉴权下载。
	VideoURL    string
	ContentPath string
	// ContentAuth 表示下载 VideoURL 时也需要携带上游鉴权头（如 Veo 文件地址）。
	ContentAuth bool
	// FileID 非空表示还需调用 Resolver 换取下载地址（如 MiniMax）。
	FileID           string
	DurationSeconds  float64
	Resolution       string
	CompletionTokens int64
	ErrorCode        string
	ErrorMessage     string
}

// NativeKind 是原生协议请求的类别。
type NativeKind string

const (
	NativeUnsupported NativeKind = ""
	NativeCreate      NativeKind = "create"
	NativeQuery       NativeKind = "query"
	NativeCancel      NativeKind = "cancel"
	// NativeFile 用于按文件 ID 取下载地址（MiniMax /v1/files/retrieve）。
	NativeFile NativeKind = "file"
	// NativeContent 是下载成品（OpenAI /v1/videos/{id}/content），由网关代理流式返回。
	NativeContent NativeKind = "content"
)

// NativeOp 是对原生协议请求的分类结果。
type NativeOp struct {
	Kind   NativeKind
	TaskID string
	FileID string
}

// Credentials 是调用上游所需的鉴权材料。
type Credentials struct {
	// BaseURL 供需要区分官方与中转的协议判断鉴权方式。
	BaseURL string
	APIKey  string
	// SecretKey 仅可灵官方 AccessKey/SecretKey 鉴权使用；为空时按 Bearer Key 发送（第三方中转）。
	SecretKey string
}

// Info 描述协议的展示与默认配置。
type Info struct {
	Protocol       Protocol `json:"protocol"`
	DisplayName    string   `json:"display_name"`
	Vendor         string   `json:"vendor"`
	DefaultBaseURL string   `json:"default_base_url"`
	// SupportsSecretKey 表示表单需要提供可选的 Secret Key 输入框。
	SupportsSecretKey bool `json:"supports_secret_key,omitempty"`
	// NativePrefix 是原生协议透传路由的前缀，如 /video/kling。
	NativePrefix string `json:"native_prefix"`
	// ExampleModels 用于表单的模型白名单建议。
	ExampleModels []string `json:"example_models,omitempty"`
}

// Provider 把统一请求与某个上游协议互相转换。实现必须是无状态、可并发调用的。
type Provider interface {
	Info() Info
	// BuildCreate 把统一请求转换为创建调用；req.Model 已是映射后的上游模型。
	BuildCreate(req Request) (Call, error)
	// Classify 识别原生协议请求（路径已去掉 NativePrefix）。
	Classify(method, path string, query url.Values) NativeOp
	// ParseNativeCreate 解析原生创建请求，返回用于路由与估价的统一请求。
	ParseNativeCreate(path string, body []byte) (Request, error)
	// RewriteNativeModel 把原生创建请求中的模型替换为上游模型，返回新的请求体与路径。
	RewriteNativeModel(path string, body []byte, model string) ([]byte, string, error)
	// ParseCreate 解析创建响应。
	ParseCreate(body []byte) (CreateResult, error)
	BuildQuery(taskID, meta string) Call
	ParseQuery(body []byte) (Snapshot, error)
	// BuildCancel 返回取消调用；协议不支持取消时第二个返回值为 false。
	BuildCancel(taskID, meta string) (Call, bool)
	// Authorize 为上游请求写入鉴权头。
	Authorize(h http.Header, cred Credentials) error
}

// FileResolver 由需要二次换取下载地址的协议实现。
type FileResolver interface {
	BuildFileQuery(fileID string) Call
	ParseFileQuery(body []byte) (string, error)
}

// CreateMetaProvider 由查询路径依赖创建路径的协议实现（如可灵按任务类型区分端点），
// 网关在提交成功后把返回值保存为任务 Meta。
type CreateMetaProvider interface {
	CreateMeta(createPath string) string
}

// ContentURLRewriter 由原生响应中携带成品地址的协议实现：网关透传原生响应前，把这些
// JSON 路径上的上游成品地址替换为网关签名链接，避免向客户端暴露上游地址。
type ContentURLRewriter interface {
	ContentURLPaths() []string
}

// NativeHeaderForwarder 由原生协议依赖特定请求头的协议实现，网关透传原生请求时
// 只转发这里列出的客户端请求头，其余头一律丢弃。
type NativeHeaderForwarder interface {
	ForwardHeaders() []string
}

// ErrUnsupported 表示协议不支持该操作。
var ErrUnsupported = errors.New("operation is not supported by this video protocol")
