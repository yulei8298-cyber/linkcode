package service

import "time"

// IntelCheckTarget 受检分组。
//
// APIKey 字段在不同层含义不同，与 ChannelMonitor 保持一致的约定：
// repository 存取的恒为密文，service 层在读出后解密、写入前加密，
// handler 层负责脱敏。公开接口永远不输出 BaseURL 与 APIKey。
type IntelCheckTarget struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	BaseURL         string    `json:"base_url"`
	APIKey          string    `json:"api_key"`
	APIMode         string    `json:"api_mode"`
	Model           string    `json:"model"`
	ReasoningEffort string    `json:"reasoning_effort"`
	RateLabel       string    `json:"rate_label"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sort_order"`
	CreatedBy       int64     `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// APIKeyDecryptFailed 标记解密失败。沿用 ChannelMonitor 的处理方式：
	// 解密失败不中断列表查询，只置位让管理端显示「需重新填写」，
	// 否则一把坏密钥会让整个列表接口 500。
	APIKeyDecryptFailed bool `json:"api_key_decrypt_failed"`
}

// IntelCheckQuestion 题库条目。逻辑题只用 ExpectedAnswer/MatchMode，
// 绘图题只用 ReferenceHTML/ReferenceMetrics/DrawingRules/ReviewRubric。
type IntelCheckQuestion struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`

	ExpectedAnswer string `json:"expected_answer"`
	MatchMode      string `json:"match_mode"`

	ReferenceHTML string `json:"reference_html"`
	// ReferenceMetrics 上传参考稿时服务端算出的结构指标快照。
	// 存 map 而非 DrawingMetrics，是因为 ent schema 无法 import service 包（循环依赖）；
	// 取用时经 DecodeDrawingMetrics 转换。
	ReferenceMetrics map[string]any `json:"reference_metrics"`
	DrawingRules     map[string]any `json:"drawing_rules"`
	ReviewRubric     string         `json:"review_rubric"`

	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IntelCheckRound 一轮检测。Seq 是对外展示的递增编号（#1284）。
type IntelCheckRound struct {
	ID                int64      `json:"id"`
	Seq               int64      `json:"seq"`
	StartedAt         time.Time  `json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	LogicQuestionID   *int64     `json:"logic_question_id"`
	DrawingQuestionID *int64     `json:"drawing_question_id"`
	TriggerSource     string     `json:"trigger_source"`
}

// IntelCheckResult 单次检测明细。
//
// 注意 RawReply / HTMLOutput 可能很大（绘图产物可达百 KB 级），
// 列表查询必须走 IntelCheckTimelinePoint 这类裁剪视图，不要整行捞。
type IntelCheckResult struct {
	ID       int64  `json:"id"`
	RoundID  int64  `json:"round_id"`
	TargetID int64  `json:"target_id"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`

	LatencyMs       *int           `json:"latency_ms"`
	PromptSnapshot  string         `json:"prompt_snapshot"`
	RawReply        string         `json:"raw_reply"`
	ExtractedAnswer string         `json:"extracted_answer"`
	HTMLOutput      string         `json:"html_output"`
	JudgeDetail     map[string]any `json:"judge_detail"`
	ErrorMessage    string         `json:"error_message"`
	InputTokens     *int           `json:"input_tokens"`
	OutputTokens    *int           `json:"output_tokens"`
	CheckedAt       time.Time      `json:"checked_at"`
}

// IntelCheckTimelinePoint 时间线色块所需的最小字段集。
//
// 单独定义是为了避免把 raw_reply / html_output 这些大字段捞进列表查询：
// 一个分组 48 格 × N 个分组，整行查询会把几十 MB 的绘图产物拖进内存，
// 而页面上每格只需要一个颜色和一个可点击的 id。
type IntelCheckTimelinePoint struct {
	ResultID  int64     `json:"result_id"`
	Status    string    `json:"status"`
	LatencyMs *int      `json:"latency_ms"`
	CheckedAt time.Time `json:"checked_at"`
}

// ---------- 公开页视图 ----------
//
// 这一组结构体是「对外能看到什么」的唯一定义。刻意不复用 IntelCheckTarget 与
// IntelCheckResult：那两个结构带着 BaseURL / APIKey / ErrorMessage，只要公开
// handler 某天图省事直接把领域模型序列化出去，上游地址与凭据就泄了。
// 字段在此逐个列出，新增字段必须显式加，漏字段只是少显示，加错字段才是事故。

// IntelCheckPublicStats 一段时间窗内的检测统计。
type IntelCheckPublicStats struct {
	Pass int64 `json:"pass"`
	Fail int64 `json:"fail"`
	// Error 为 request_error 条数。它不进通过率的分母——上游抖动不是模型能力问题。
	Error int64 `json:"error"`
	// Unverified 为产物存在但动作证据不足的条数，同样不进通过率分母。
	Unverified int64 `json:"unverified"`

	PassRate float64 `json:"pass_rate"`
	// HasData 为 false 时前端显示「暂无数据」而非 0%，二者含义完全不同。
	HasData bool `json:"has_data"`
}

// IntelCheckPublicGroup 公开页上的一张分组卡片。
//
// Model 与 ReasoningEffort 是有意公开的：本页要证明的正是「跑的是满血配置」，
// 把模型名与推理等级藏起来，页面就失去了可验证性。BaseURL / APIKey / APIMode
// 与 CreatedBy 不在此列——前者是凭据，APIMode 属于 HTTP 细节。
type IntelCheckPublicGroup struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	RateLabel       string `json:"rate_label"`

	// State 取值见 IntelCheckState 常量，由 ReplayIntelCheckState 从逻辑题时间线回放得出。
	State string `json:"state"`

	// 两条时间线均按 checked_at 升序，与页面从左到右的排列一致。
	LogicTimeline   []*IntelCheckTimelinePoint `json:"logic_timeline"`
	DrawingTimeline []*IntelCheckTimelinePoint `json:"drawing_timeline"`

	LogicStats24h   IntelCheckPublicStats `json:"logic_stats_24h"`
	DrawingStats24h IntelCheckPublicStats `json:"drawing_stats_24h"`

	// LatestDrawingResultID 最近一次有画作产出的明细 id，0 表示暂无。
	// 判失败的画作同样会出现在这里——让用户看到降智时画成什么样，正是本页的说服力来源。
	LatestDrawingResultID int64 `json:"latest_drawing_result_id"`
}

// IntelCheckPublicSummary 页面顶部的汇总。
type IntelCheckPublicSummary struct {
	TotalGroups    int `json:"total_groups"`
	NormalGroups   int `json:"normal_groups"`
	DegradedGroups int `json:"degraded_groups"`
	UnknownGroups  int `json:"unknown_groups"`

	// Stats24h 为全部分组、全部题型的合计。
	Stats24h IntelCheckPublicStats `json:"stats_24h"`

	LatestRoundSeq int64      `json:"latest_round_seq"`
	LastCheckedAt  *time.Time `json:"last_checked_at"`
	// NextCheckAt 是按「上一轮开始时间 + interval_minutes」推出的预计时刻，不是承诺。
	// ticker 的相位随进程启动漂移（见设计文档 §4.2），进程刚重启时会偏；
	// 若该时刻已过去仍照实返回，不做美化——那恰好说明调度停了，藏起来只会让故障更难发现。
	NextCheckAt *time.Time `json:"next_check_at"`
}

// IntelCheckPublicOverview 公开页一次请求拿到的全部数据。
type IntelCheckPublicOverview struct {
	IntroTitle string `json:"intro_title"`
	IntroText  string `json:"intro_text"`

	IntervalMinutes int `json:"interval_minutes"`
	TimelinePoints  int `json:"timeline_points"`
	// DegradedRule 一并返回，让页面能把「连续 3 次失败判降智」这条规则明写出来。
	// 判定规则不公开，色块就只是颜色，用户无从复核。
	DegradedRule IntelCheckDegradedRule `json:"degraded_rule"`

	Summary IntelCheckPublicSummary  `json:"summary"`
	Groups  []*IntelCheckPublicGroup `json:"groups"`

	// GeneratedAt 服务端生成时刻，供前端以服务端时间为基准做倒计时，
	// 避免客户端时钟偏差把倒计时算歪。
	GeneratedAt time.Time `json:"generated_at"`
}

// IntelCheckPublicResult 单次检测详情的对外视图。
//
// 相比 IntelCheckResult 少了三样东西，都是刻意的：RoundID（内部主键，对外只给 Seq）、
// ErrorMessage（可能带上游域名、状态码与凭据片段，一律换成按状态生成的中性文案 StatusNote）、
// InputTokens/OutputTokens（属于计费口径，与「模型是否满血」无关，公开出去只会引出无谓的质疑）。
type IntelCheckPublicResult struct {
	ID       int64  `json:"id"`
	RoundSeq int64  `json:"round_seq"`
	Kind     string `json:"kind"`
	Status   string `json:"status"`

	TargetID        int64  `json:"target_id"`
	TargetName      string `json:"target_name"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`

	// QuestionTitle 与 PromptSnapshot 分别是题目名与本轮实际发送的题面。
	QuestionTitle  string `json:"question_title"`
	PromptSnapshot string `json:"prompt_snapshot"`

	// 逻辑题专用：期望答案、匹配模式与提取到的答案，三者合起来才能让用户复核判定。
	ExpectedAnswer  string `json:"expected_answer"`
	MatchMode       string `json:"match_mode"`
	ExtractedAnswer string `json:"extracted_answer"`

	RawReply string `json:"raw_reply"`

	// 绘图题专用：清洗后的 HTML 与逐项判定明细。
	HTMLOutput  string         `json:"html_output"`
	JudgeDetail map[string]any `json:"judge_detail"`

	LatencyMs *int `json:"latency_ms"`
	// StatusNote 状态的中性说明，取代 error_message。
	StatusNote string    `json:"status_note"`
	CheckedAt  time.Time `json:"checked_at"`
}

// IntelCheckTargetListParams 受检分组列表查询参数。
type IntelCheckTargetListParams struct {
	Page     int
	PageSize int
	// Enabled 为 nil 表示不按启用状态过滤。
	Enabled *bool
	Search  string
}

// IntelCheckQuestionListParams 题库列表查询参数。
type IntelCheckQuestionListParams struct {
	Page     int
	PageSize int
	// Kind 为空表示不限题型。
	Kind    string
	Enabled *bool
}

// IntelCheckResultListParams 明细列表查询参数（管理端排障用）。
type IntelCheckResultListParams struct {
	Page     int
	PageSize int
	RoundID  *int64
	TargetID *int64
	Kind     string
	Status   string
}

// IntelCheckRoundListParams 轮次列表查询参数。
type IntelCheckRoundListParams struct {
	Page     int
	PageSize int
}

// IntelCheckResultDraft 创建 running 占位行所需的字段。
//
// 发起上游请求前先批量落 running 行，前端立刻能看到「检测中」浅蓝块；
// 拿到结果后按 (RoundID, TargetID, Kind) 原地更新。
type IntelCheckResultDraft struct {
	RoundID        int64
	TargetID       int64
	Kind           string
	PromptSnapshot string
	CheckedAt      time.Time
}

// IntelCheckResultOutcome 一次检测的最终结果，用于原地更新 running 行。
type IntelCheckResultOutcome struct {
	RoundID  int64
	TargetID int64
	Kind     string

	Status          string
	LatencyMs       *int
	RawReply        string
	ExtractedAnswer string
	HTMLOutput      string
	JudgeDetail     map[string]any
	ErrorMessage    string
	InputTokens     *int
	OutputTokens    *int
	CheckedAt       time.Time
}
