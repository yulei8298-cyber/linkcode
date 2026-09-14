package service

import (
	"context"
	"fmt"
	"strings"
)

// 管理端分页参数。
const (
	// intelCheckDefaultPageSize 未指定时的每页条数。
	intelCheckDefaultPageSize = 20

	// IntelCheckMaxPageSize 轮次等轻量列表的每页上限。
	IntelCheckMaxPageSize = 100

	// IntelCheckResultMaxPageSize 明细列表的每页上限，刻意远低于上面那个。
	//
	// 明细行带着 raw_reply 与 html_output，绘图产物动辄几百 KB，
	// 按 100 条一页捞等于一次查询往内存里拖几十 MB。管理端看明细是逐条排障，
	// 20 条足够；真要看更多也该翻页，而不是把单页加大。
	//
	// 导出是为了让 handler 能按同一个常量先收敛一次再调用：分页响应要回报
	// 实际生效的 pageSize，否则前端拿请求值去算总页数，翻到后面全是空页。
	// 两层都收敛看似重复，但常量只有一个，不存在漂移。
	IntelCheckResultMaxPageSize = 20
)

// IntelCheckTrialResult 管理端试跑与阈值标定的结果。
//
// 与 IntelCheckResultOutcome 的关键差别是没有 RoundID：试跑不落库、不占轮次号，
// 也就不该伪装成一条检测记录出现在时间线上。
//
// ErrorMessage 在这里对管理员可见——那是排障所需，与公开详情必须脱敏
// 并不冲突：这个结构只经由 adminAuth 后的接口返回，不进 IntelCheckPublicResult。
type IntelCheckTrialResult struct {
	QuestionID    int64  `json:"question_id"`
	QuestionKind  string `json:"question_kind"`
	QuestionTitle string `json:"question_title"`

	// 以下四项仅在试跑（DryRunTarget）时有值；粘贴产物判定没有受检分组可言。
	TargetID        int64  `json:"target_id"`
	TargetName      string `json:"target_name"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`

	Status          string         `json:"status"`
	LatencyMs       *int           `json:"latency_ms"`
	RawReply        string         `json:"raw_reply"`
	ExtractedAnswer string         `json:"extracted_answer"`
	HTMLOutput      string         `json:"html_output"`
	JudgeDetail     map[string]any `json:"judge_detail"`
	ErrorMessage    string         `json:"error_message"`
	InputTokens     *int           `json:"input_tokens"`
	OutputTokens    *int           `json:"output_tokens"`
}

// normalizeIntelCheckPaging 收敛分页参数到合法区间。
//
// 与设置项一样采取「静默收敛」而非报错：管理端传了 pageSize=1000
// 时返回上限条数比回一个 400 更有用，调用方本来也只是想多看几条。
func normalizeIntelCheckPaging(page, pageSize, maxPageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = intelCheckDefaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// ListRounds 分页查询检测轮次（按 seq 降序，由仓储保证）。
func (s *IntelCheckService) ListRounds(
	ctx context.Context, params IntelCheckRoundListParams,
) ([]*IntelCheckRound, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, fmt.Errorf("智力检测服务未初始化")
	}
	params.Page, params.PageSize = normalizeIntelCheckPaging(
		params.Page, params.PageSize, IntelCheckMaxPageSize)

	items, total, err := s.repo.ListRounds(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check rounds: %w", err)
	}
	return items, total, nil
}

// ListResults 分页查询检测明细，供管理端排障。
//
// 返回的是完整领域对象（含 ErrorMessage 与上游报错原文）。handler 必须区分对待：
// 管理端接口可以照实输出，公开接口只能走 PublicResultDetail。
func (s *IntelCheckService) ListResults(
	ctx context.Context, params IntelCheckResultListParams,
) ([]*IntelCheckResult, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, fmt.Errorf("智力检测服务未初始化")
	}
	params.Page, params.PageSize = normalizeIntelCheckPaging(
		params.Page, params.PageSize, IntelCheckResultMaxPageSize)

	items, total, err := s.repo.ListResults(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check results: %w", err)
	}
	return items, total, nil
}

// EvaluateDrawingSource 只对一份现成产物跑判定，不向受检分组发绘图请求。
//
// 用途是标定阈值：管理员把一份已知好坏的 HTML 粘进来，立刻看到结构门禁的逐项
// 结果与源码评审得分，据此调 min_ratio 与 pass_score（设计文档 §5.2）。
// 省掉绘图那次请求，标定一次的成本从几分钟降到一次评审调用。
//
// 刻意不检查总开关：ValidateIntelCheckSettings 要求开启前必须先配好评审模型，
// 而配之前恰恰需要这个工具验证阈值合不合适——加开关检查会把标定卡在开启之前，
// 逼管理员先带着未验证的阈值上线。
func (s *IntelCheckService) EvaluateDrawingSource(
	ctx context.Context, questionID int64, source string,
) (*IntelCheckTrialResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("智力检测服务未初始化")
	}
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("待判定的产物内容不能为空")
	}

	question, err := s.GetQuestion(ctx, questionID)
	if err != nil {
		return nil, err
	}
	// 逻辑题没有产物可判。这里报错而不是回落成答案匹配：调用方显然拿错了题目 id，
	// 静默换一条判定路径只会让标定结果无从解释。
	if question.Kind != IntelCheckKindDrawing {
		return nil, fmt.Errorf("只有绘图题支持粘贴产物判定，该题题型为 %s", question.Kind)
	}
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	// 关闭了评审就不去加载评审分组：那次加载会在未配置时打一条 Warn 日志，
	// 而"没配"在这个模式下是正常状态，不该被记成异常。
	var judge *IntelCheckTarget
	if !cfg.DrawingJudge.SkipReview {
		judge = s.loadIntelCheckJudgeTarget(ctx, cfg)
	}
	judged := s.judgeIntelCheckDrawing(ctx, cfg, question, judge, source)

	return &IntelCheckTrialResult{
		QuestionID:    question.ID,
		QuestionKind:  question.Kind,
		QuestionTitle: question.Title,
		Status:        judged.Status,
		// 原样回显提交内容：与 HTMLOutput（清洗后）并排看，
		// 管理员才能判断某项门禁是败在模型产出还是败在清洗环节。
		RawReply:     source,
		HTMLOutput:   judged.HTMLOutput,
		JudgeDetail:  judged.JudgeDetail,
		ErrorMessage: judged.ErrorMessage,
	}, nil
}

// DryRunTarget 对单个分组试跑一题，结果不落库。
//
// 同样不检查总开关，也不要求分组或题目处于启用状态：这个接口存在的意义就是
// 「先验证配置对不对，再开启 / 再启用」。
func (s *IntelCheckService) DryRunTarget(
	ctx context.Context, questionID, targetID int64,
) (*IntelCheckTrialResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("智力检测服务未初始化")
	}

	question, err := s.GetQuestion(ctx, questionID)
	if err != nil {
		return nil, err
	}
	target, err := s.GetTarget(ctx, targetID)
	if err != nil {
		return nil, err
	}
	// 试跑要真发上游请求，凭据解不开就无从「跑一次看看」。
	// 这里直接报错而不是记成 request_error：真实轮次里跳过该分组是对的
	// （不能让一把坏密钥牵连全局），但管理员主动点试跑时，
	// 他要的正是失败原因，回一个黄色的「请求未完成」等于把答案藏起来。
	if target.APIKeyDecryptFailed {
		return nil, fmt.Errorf("该分组的 API Key 无法解密，请重新填写后再试跑")
	}
	cfg, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	var judge *IntelCheckTarget
	if question.Kind == IntelCheckKindDrawing {
		judge = s.loadIntelCheckJudgeTarget(ctx, cfg)
	}

	// 与真实轮次共用 probeIntelCheckOnce：试跑的结论必须与线上判定一致，
	// 否则这个接口就是个会骗人的工具。
	probe := s.probeIntelCheckOnce(ctx, cfg, target, question, judge)

	out := &IntelCheckTrialResult{
		QuestionID:      question.ID,
		QuestionKind:    question.Kind,
		QuestionTitle:   question.Title,
		TargetID:        target.ID,
		TargetName:      target.Name,
		Model:           target.Model,
		ReasoningEffort: target.ReasoningEffort,
		Status:          probe.Judged.Status,
		ExtractedAnswer: probe.Judged.ExtractedAnswer,
		HTMLOutput:      probe.Judged.HTMLOutput,
		JudgeDetail:     probe.Judged.JudgeDetail,
		ErrorMessage:    probe.Judged.ErrorMessage,
	}
	if probe.Reply != nil {
		out.LatencyMs = &probe.Reply.LatencyMs
		out.InputTokens = probe.Reply.InputTokens
		out.OutputTokens = probe.Reply.OutputTokens
		out.RawReply = probe.Reply.Text
	}
	return out, nil
}
