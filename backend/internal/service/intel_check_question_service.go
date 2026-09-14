package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// 题库字段上限。前两项与迁移 238 的列宽一致；后三项落在 TEXT 列上，
// 是存储与解析的防御性上限。新版仅将题面发往受检模型，标准原文不发上游。
const (
	maxIntelCheckQuestionTitleRunes  = 100
	maxIntelCheckQuestionAnswerRunes = 500

	maxIntelCheckQuestionPromptRunes = 20000
	maxIntelCheckReviewRubricRunes   = 20000

	// maxIntelCheckReferenceHTMLBytes 取候选稿门禁上限（256 KiB）的两倍。
	// 标准由管理员上传，但仍需限制存储体积与静态解析的资源开销。
	maxIntelCheckReferenceHTMLBytes = 512 * 1024
)

// IntelCheckQuestionParams 题库条目的创建/更新入参。
//
// 逻辑题只用 ExpectedAnswer/MatchMode，绘图题只用 ReferenceHTML/DrawingRules/
// ReviewRubric，跨题型的字段会在写入时被清空。
//
// 更新是整块覆盖而非逐字段合并：管理端表单一次提交题目全部字段，且这些字段
// 都能原样回填（与受检分组的 APIKey 不同，后者展示的是掩码、无法回传）。
type IntelCheckQuestionParams struct {
	Kind   string
	Title  string
	Prompt string

	ExpectedAnswer string
	MatchMode      string

	ReferenceHTML string
	DrawingRules  map[string]any
	ReviewRubric  string

	Enabled bool
}

// normalizeIntelCheckQuestionKind 校验题型，不做回落。
//
// 题型决定走哪条判定链路，猜错会让一道绘图题被当成逻辑题去比字符串，
// 判定结果恒为 fail，而页面上看不出任何配置问题。
func normalizeIntelCheckQuestionKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case IntelCheckKindLogic:
		return IntelCheckKindLogic, nil
	case IntelCheckKindDrawing:
		return IntelCheckKindDrawing, nil
	default:
		return "", fmt.Errorf("题型只能是 %s 或 %s", IntelCheckKindLogic, IntelCheckKindDrawing)
	}
}

// normalizeIntelCheckMatchMode 校验答案匹配模式；留空取 exact。
//
// 留空回落 exact 与迁移的列默认值一致，但填了无法识别的值必须报错：静默回落
// 会把 contains 的拼写错误变成严格全等，判定从「通过」翻成「失败」。
func normalizeIntelCheckMatchMode(mode string) (string, error) {
	switch mode = strings.ToLower(strings.TrimSpace(mode)); mode {
	case "":
		return IntelCheckMatchExact, nil
	case IntelCheckMatchExact, IntelCheckMatchNumeric, IntelCheckMatchContains, IntelCheckMatchRegex:
		return mode, nil
	default:
		return "", fmt.Errorf("答案匹配模式只能是 %s / %s / %s / %s",
			IntelCheckMatchExact, IntelCheckMatchNumeric, IntelCheckMatchContains, IntelCheckMatchRegex)
	}
}

// buildIntelCheckQuestion 校验入参并写入领域对象。
func buildIntelCheckQuestion(q *IntelCheckQuestion, p IntelCheckQuestionParams) error {
	kind, err := normalizeIntelCheckQuestionKind(p.Kind)
	if err != nil {
		return err
	}

	title := strings.TrimSpace(p.Title)
	if title == "" {
		return fmt.Errorf("题目标题不能为空")
	}
	if n := len([]rune(title)); n > maxIntelCheckQuestionTitleRunes {
		return fmt.Errorf("题目标题过长：%d 字，上限 %d 字", n, maxIntelCheckQuestionTitleRunes)
	}

	prompt := strings.TrimSpace(p.Prompt)
	if prompt == "" {
		return fmt.Errorf("题面不能为空")
	}
	if n := len([]rune(prompt)); n > maxIntelCheckQuestionPromptRunes {
		return fmt.Errorf("题面过长：%d 字，上限 %d 字", n, maxIntelCheckQuestionPromptRunes)
	}

	q.Kind = kind
	q.Title = title
	q.Prompt = prompt
	q.Enabled = p.Enabled

	if kind == IntelCheckKindLogic {
		return applyIntelCheckLogicFields(q, p)
	}
	return applyIntelCheckDrawingFields(q, p)
}

// applyIntelCheckLogicFields 写入逻辑题字段，并清空绘图题专属字段。
//
// 清空而非保留：题型改成 logic 后若留着旧参考稿与门禁规则，管理端表单会显示
// 一堆不生效的配置，日后改回 drawing 时又会悄悄套用过时的参考指标。
func applyIntelCheckLogicFields(q *IntelCheckQuestion, p IntelCheckQuestionParams) error {
	answer := strings.TrimSpace(p.ExpectedAnswer)
	if answer == "" {
		return fmt.Errorf("逻辑题必须填写标准答案")
	}
	if n := len([]rune(answer)); n > maxIntelCheckQuestionAnswerRunes {
		return fmt.Errorf("标准答案过长：%d 字，上限 %d 字", n, maxIntelCheckQuestionAnswerRunes)
	}

	mode, err := normalizeIntelCheckMatchMode(p.MatchMode)
	if err != nil {
		return err
	}
	// 正则在这里就编译一次。留到判定时才发现写错，那一轮所有分组都会记成
	// fail，而真正的原因（正则非法）只留在日志里。
	if mode == IntelCheckMatchRegex {
		if _, err := regexp.Compile(answer); err != nil {
			return fmt.Errorf("正则匹配模式下，标准答案必须是合法正则：%v", err)
		}
	}

	q.ExpectedAnswer = answer
	q.MatchMode = mode
	q.ReferenceHTML = ""
	q.ReferenceMetrics = nil
	q.DrawingRules = nil
	q.ReviewRubric = ""
	return nil
}

// applyIntelCheckDrawingFields 写入绘图题字段，算出参考指标并清空逻辑题专属字段。
//
// reference_metrics 一律由服务端从 reference_html 现算，不接受调用方传入：
// 它是门禁全部相对项的分母，可写等于把判定阈值交给调用方。
func applyIntelCheckDrawingFields(q *IntelCheckQuestion, p IntelCheckQuestionParams) error {
	rubric := strings.TrimSpace(p.ReviewRubric)
	if n := len([]rune(rubric)); n > maxIntelCheckReviewRubricRunes {
		return fmt.Errorf("评审清单过长：%d 字，上限 %d 字", n, maxIntelCheckReviewRubricRunes)
	}

	reference := strings.TrimSpace(p.ReferenceHTML)
	if len(reference) > maxIntelCheckReferenceHTMLBytes {
		return fmt.Errorf("参考稿过大：%d 字节，上限 %d 字节",
			len(reference), maxIntelCheckReferenceHTMLBytes)
	}

	var metrics map[string]any
	if reference != "" {
		computed, err := ComputeDrawingMetrics(reference)
		if err != nil {
			// 不接受算不出指标的参考稿：门禁在参考值为 0 时会跳过对应的相对项，
			// 存一份解析不了的参考稿等于悄悄关掉整层结构门禁。
			return fmt.Errorf("参考稿无法解析出结构指标：%v", err)
		}
		encoded, err := EncodeDrawingMetrics(computed)
		if err != nil {
			return err
		}
		metrics = encoded
	}

	rules, err := DecodeIntelCheckDrawingRules(p.DrawingRules)
	if err != nil {
		return err
	}
	if reference != "" || len(rules.StandardSources) > 0 {
		baseline, count, err := intelCheckStructureBaseline(reference, rules.StandardSources)
		if err != nil {
			return err
		}
		if metrics == nil {
			metrics = make(map[string]any)
		}
		metrics["structure_baseline"] = baseline
		metrics["standard_count"] = count
	}
	// 落库前就归一化，使管理端读回的规则与判定时实际生效的规则完全一致。
	rules.Normalize()
	encodedRules, err := EncodeIntelCheckDrawingRules(rules)
	if err != nil {
		return err
	}

	q.ReferenceHTML = reference
	q.ReferenceMetrics = metrics
	q.DrawingRules = encodedRules
	q.ReviewRubric = rubric
	q.ExpectedAnswer = ""
	// 绘图题不用匹配模式，但该列 NOT NULL 且带 CHECK 约束，
	// 只能写一个合法值，不能留空串。
	q.MatchMode = IntelCheckMatchExact
	return nil
}

// ---------- 题库 CRUD ----------

// ListQuestions 分页查询题库。
func (s *IntelCheckService) ListQuestions(
	ctx context.Context, params IntelCheckQuestionListParams,
) ([]*IntelCheckQuestion, int64, error) {
	items, total, err := s.repo.ListQuestions(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check questions: %w", err)
	}
	return items, total, nil
}

// GetQuestion 查询单道题目。
func (s *IntelCheckService) GetQuestion(ctx context.Context, id int64) (*IntelCheckQuestion, error) {
	return s.repo.GetQuestionByID(ctx, id)
}

// CreateQuestion 新增题目，绘图题会同时算出并返回 reference_metrics 供管理员核对。
func (s *IntelCheckService) CreateQuestion(
	ctx context.Context, p IntelCheckQuestionParams,
) (*IntelCheckQuestion, error) {
	q := &IntelCheckQuestion{}
	if err := buildIntelCheckQuestion(q, p); err != nil {
		return nil, err
	}
	if err := s.repo.CreateQuestion(ctx, q); err != nil {
		return nil, fmt.Errorf("create intel check question: %w", err)
	}
	return q, nil
}

// UpdateQuestion 整块覆盖题目。
//
// 不先读原行：题目的每个字段都由管理端完整提交，读回来也会被全量覆盖，
// 多一次查询只是多一次往返。行不存在由 UpdateOneID 报 ErrIntelCheckQuestionNotFound。
func (s *IntelCheckService) UpdateQuestion(
	ctx context.Context, id int64, p IntelCheckQuestionParams,
) (*IntelCheckQuestion, error) {
	q := &IntelCheckQuestion{ID: id}
	if err := buildIntelCheckQuestion(q, p); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateQuestion(ctx, q); err != nil {
		return nil, err
	}
	return q, nil
}

// DeleteQuestion 删除题目。
//
// rounds 的两个 question_id 是 ON DELETE SET NULL，历史轮次不受影响：
// 题面已快照在 results.prompt_snapshot 里，删题不会让旧详情失去上下文。
func (s *IntelCheckService) DeleteQuestion(ctx context.Context, id int64) error {
	if err := s.repo.DeleteQuestion(ctx, id); err != nil {
		return err
	}
	return nil
}

// PickEnabledQuestion 取指定题型中一道启用的题目，供 runner 组织单轮检测。
func (s *IntelCheckService) PickEnabledQuestion(ctx context.Context, kind string) (*IntelCheckQuestion, error) {
	normalized, err := normalizeIntelCheckQuestionKind(kind)
	if err != nil {
		return nil, err
	}
	return s.repo.PickEnabledQuestion(ctx, normalized)
}
