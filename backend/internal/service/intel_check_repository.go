package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 智力检测的哨兵错误。
//
// 用 internal/pkg/errors 的语义化构造函数（NotFound/BadRequest 等），与
// setting_service.go 等既有服务保持一致；状态码由构造函数决定，handler 层用
// infraerrors.Code(err) 取出即可，不必维护错误到状态码的映射表。
//
// 这些值可以直接用 errors.Is 比较：ApplicationError.Is 按 Code + Reason 判等而非
// 指针相等，因此经过 WithCause/Clone 包装后依然能命中。
var (
	ErrIntelCheckTargetNotFound = infraerrors.NotFound(
		"INTEL_CHECK_TARGET_NOT_FOUND", "受检分组不存在")
	ErrIntelCheckQuestionNotFound = infraerrors.NotFound(
		"INTEL_CHECK_QUESTION_NOT_FOUND", "题目不存在")
	ErrIntelCheckResultNotFound = infraerrors.NotFound(
		"INTEL_CHECK_RESULT_NOT_FOUND", "检测记录不存在")
	ErrIntelCheckRoundNotFound = infraerrors.NotFound(
		"INTEL_CHECK_ROUND_NOT_FOUND", "检测轮次不存在")

	// ErrIntelCheckDisabled 功能未开启。这里刻意用 NotFound 而非 Forbidden：
	// 未开启时不应让外部探知该功能是否存在，403 等于承认「有但不给你看」。
	ErrIntelCheckDisabled = infraerrors.NotFound(
		"INTEL_CHECK_DISABLED", "功能未开启")

	// ErrIntelCheckNoQuestion 题库里没有该题型的启用题目。
	// 400 而非 500：这是配置缺失，管理员补一道题即可，不是系统故障。
	ErrIntelCheckNoQuestion = infraerrors.BadRequest(
		"INTEL_CHECK_NO_QUESTION", "题库中没有启用的该类型题目")
)

// IntelCheckRepository 智力检测数据访问接口。
//
// 与 ChannelMonitorRepository 保持一致的约定：入参与返回均使用 service 包的
// 模型，repository 实现负责与 ent 模型互转，并保持 APIKey 字段恒为密文——
// 加解密只发生在 service 层。
type IntelCheckRepository interface {
	// ---------- 受检分组 ----------

	CreateTarget(ctx context.Context, target *IntelCheckTarget) error
	GetTargetByID(ctx context.Context, id int64) (*IntelCheckTarget, error)
	UpdateTarget(ctx context.Context, target *IntelCheckTarget) error
	DeleteTarget(ctx context.Context, id int64) error
	ListTargets(ctx context.Context, params IntelCheckTargetListParams) ([]*IntelCheckTarget, int64, error)
	// ListEnabledTargets 按 sort_order 升序返回全部启用分组，供调度器与公开页使用。
	ListEnabledTargets(ctx context.Context) ([]*IntelCheckTarget, error)

	// ---------- 题库 ----------

	CreateQuestion(ctx context.Context, question *IntelCheckQuestion) error
	GetQuestionByID(ctx context.Context, id int64) (*IntelCheckQuestion, error)
	UpdateQuestion(ctx context.Context, question *IntelCheckQuestion) error
	DeleteQuestion(ctx context.Context, id int64) error
	ListQuestions(ctx context.Context, params IntelCheckQuestionListParams) ([]*IntelCheckQuestion, int64, error)
	// PickEnabledQuestion 取指定题型中一道启用的题目。
	// 题库只有一道时即固定返回它；一道都没有时返回 ErrIntelCheckNoQuestion。
	PickEnabledQuestion(ctx context.Context, kind string) (*IntelCheckQuestion, error)

	// ---------- 轮次 ----------

	// CreateRound 创建轮次并回填 ID 与 Seq。
	//
	// Seq 由 intel_check_rounds_seq_seq 序列发号，调用方不要自行赋值：
	// 它是对外展示的累计轮次号，必须不受保留期清理影响（见迁移 239）。
	CreateRound(ctx context.Context, round *IntelCheckRound) error
	FinishRound(ctx context.Context, id int64, finishedAt time.Time) error
	// GetRoundByID 取单轮，供详情接口经 round 回溯本次用的是哪道题。
	//
	// 明细行只存了题面快照，没存标准答案；而公开页详情要展示「期望答案 vs 提取答案」，
	// 就必须沿 result → round → question 这条链走。不把答案冗余进明细行，是因为
	// 题目的标准答案改了之后，历史明细该显示当时那道题的答案还是现在的，没有唯一正解，
	// 而经 round 回溯至少语义明确：显示的就是当前题库里那道题的答案。
	GetRoundByID(ctx context.Context, id int64) (*IntelCheckRound, error)
	ListRounds(ctx context.Context, params IntelCheckRoundListParams) ([]*IntelCheckRound, int64, error)

	// ---------- 明细 ----------

	// CreateRunningResults 批量插入 running 占位行。
	//
	// 必须按 (round_id, target_id, kind) 唯一索引做 upsert：调度器重试或
	// 管理端在同一轮次重复触发时，不能让时间线凭空多出色块。
	CreateRunningResults(ctx context.Context, drafts []*IntelCheckResultDraft) error
	// SaveResultOutcome 按 (round_id, target_id, kind) 原地更新占位行为最终结果。
	SaveResultOutcome(ctx context.Context, outcome *IntelCheckResultOutcome) error
	GetResultByID(ctx context.Context, id int64) (*IntelCheckResult, error)
	ListResults(ctx context.Context, params IntelCheckResultListParams) ([]*IntelCheckResult, int64, error)

	// ---------- 公开页聚合 ----------

	// ListTimelineForTargets 批量取多个分组各题型最近 limit 条的时间线色块。
	//
	// 返回值按 targetID → kind → 色块切片组织，色块按 checked_at 升序（旧→新），
	// 与页面从左到右的排列一致。只查色块所需的最小字段集，
	// 不得把 raw_reply / html_output 捞进来——那会让一次页面请求拖进几十 MB。
	ListTimelineForTargets(ctx context.Context, targetIDs []int64, limit int) (map[int64]map[string][]*IntelCheckTimelinePoint, error)
	// LatestDrawingResultIDs 取各分组最近一次「有画作产出」的 result id，
	// 供公开页右侧展示最新画作。没有产出的分组不出现在返回值里。
	LatestDrawingResultIDs(ctx context.Context, targetIDs []int64) (map[int64]int64, error)
	// CountByStatusSince 统计各分组自 since 起各状态的条数，用于算通过率。
	// 返回值按 targetID → status → 条数组织。
	CountByStatusSince(ctx context.Context, targetIDs []int64, kind string, since time.Time) (map[int64]map[string]int64, error)

	// ---------- 留存清理 ----------

	// DeleteResultsBefore 删除 checked_at 早于 before 的明细，返回删除行数。
	DeleteResultsBefore(ctx context.Context, before time.Time) (int64, error)
	// DeleteEmptyRoundsBefore 删除 started_at 早于 before 且已无明细的轮次。
	// 必须在 DeleteResultsBefore 之后调用，否则外键级联会连带删掉尚在留存期的明细。
	DeleteEmptyRoundsBefore(ctx context.Context, before time.Time) (int64, error)
}
