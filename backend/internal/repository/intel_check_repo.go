package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/intelcheckquestion"
	"github.com/Wei-Shaw/sub2api/ent/intelchecktarget"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// intelCheckRepository 实现 service.IntelCheckRepository。
//
// 选型与 channelMonitorRepository 一致：配置表的 CRUD 走 ent（复用事务上下文），
// 公开页那几个「每组取最近 N 条」「按状态分组计数」的聚合查询走原生 SQL——
// 它们需要窗口函数与 DISTINCT ON，用 ent 拼既啰嗦又容易让索引失效。
//
// APIKey 在本层恒为密文：加解密只发生在 service 层，仓储不持有密钥。
type intelCheckRepository struct {
	client *dbent.Client
	db     *sql.DB
}

// NewIntelCheckRepository 创建仓储实例。
func NewIntelCheckRepository(client *dbent.Client, db *sql.DB) service.IntelCheckRepository {
	return &intelCheckRepository{client: client, db: db}
}

// intelCheckPaging 归一化分页参数，与既有仓储保持同一套默认值。
func intelCheckPaging(page, pageSize int) (int, int) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if page <= 0 {
		page = 1
	}
	return page, pageSize
}

// ---------- 受检分组 ----------

func (r *intelCheckRepository) CreateTarget(ctx context.Context, t *service.IntelCheckTarget) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.IntelCheckTarget.Create().
		SetName(t.Name).
		SetDescription(t.Description).
		SetBaseURL(t.BaseURL).
		SetAPIKeyEncrypted(t.APIKey). // 调用方传入的已是密文
		SetAPIMode(t.APIMode).
		SetModel(t.Model).
		SetReasoningEffort(t.ReasoningEffort).
		SetRateLabel(t.RateLabel).
		SetEnabled(t.Enabled).
		SetSortOrder(t.SortOrder).
		SetCreatedBy(t.CreatedBy).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckTargetNotFound, nil)
	}
	t.ID = created.ID
	t.CreatedAt = created.CreatedAt
	t.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *intelCheckRepository) GetTargetByID(ctx context.Context, id int64) (*service.IntelCheckTarget, error) {
	row, err := r.client.IntelCheckTarget.Query().
		Where(intelchecktarget.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrIntelCheckTargetNotFound, nil)
	}
	return entToIntelCheckTarget(row), nil
}

func (r *intelCheckRepository) UpdateTarget(ctx context.Context, t *service.IntelCheckTarget) error {
	client := clientFromContext(ctx, r.client)
	updated, err := client.IntelCheckTarget.UpdateOneID(t.ID).
		SetName(t.Name).
		SetDescription(t.Description).
		SetBaseURL(t.BaseURL).
		SetAPIKeyEncrypted(t.APIKey).
		SetAPIMode(t.APIMode).
		SetModel(t.Model).
		SetReasoningEffort(t.ReasoningEffort).
		SetRateLabel(t.RateLabel).
		SetEnabled(t.Enabled).
		SetSortOrder(t.SortOrder).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckTargetNotFound, nil)
	}
	t.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *intelCheckRepository) DeleteTarget(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.IntelCheckTarget.DeleteOneID(id).Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckTargetNotFound, nil)
	}
	return nil
}

func (r *intelCheckRepository) ListTargets(
	ctx context.Context, params service.IntelCheckTargetListParams,
) ([]*service.IntelCheckTarget, int64, error) {
	q := r.client.IntelCheckTarget.Query()
	if params.Enabled != nil {
		q = q.Where(intelchecktarget.EnabledEQ(*params.Enabled))
	}
	if s := strings.TrimSpace(params.Search); s != "" {
		q = q.Where(intelchecktarget.Or(
			intelchecktarget.NameContainsFold(s),
			intelchecktarget.ModelContainsFold(s),
		))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count intel check targets: %w", err)
	}

	page, pageSize := intelCheckPaging(params.Page, params.PageSize)
	rows, err := q.
		Order(dbent.Asc(intelchecktarget.FieldSortOrder), dbent.Asc(intelchecktarget.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check targets: %w", err)
	}

	items := make([]*service.IntelCheckTarget, 0, len(rows))
	for _, row := range rows {
		items = append(items, entToIntelCheckTarget(row))
	}
	return items, int64(total), nil
}

func (r *intelCheckRepository) ListEnabledTargets(ctx context.Context) ([]*service.IntelCheckTarget, error) {
	rows, err := r.client.IntelCheckTarget.Query().
		Where(intelchecktarget.EnabledEQ(true)).
		Order(dbent.Asc(intelchecktarget.FieldSortOrder), dbent.Asc(intelchecktarget.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled intel check targets: %w", err)
	}
	items := make([]*service.IntelCheckTarget, 0, len(rows))
	for _, row := range rows {
		items = append(items, entToIntelCheckTarget(row))
	}
	return items, nil
}

// ---------- 题库 ----------

func (r *intelCheckRepository) CreateQuestion(ctx context.Context, q *service.IntelCheckQuestion) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.IntelCheckQuestion.Create().
		SetKind(intelcheckquestion.Kind(q.Kind)).
		SetTitle(q.Title).
		SetPrompt(q.Prompt).
		SetExpectedAnswer(q.ExpectedAnswer).
		SetMatchMode(q.MatchMode).
		SetReferenceHTML(q.ReferenceHTML).
		SetReferenceMetrics(q.ReferenceMetrics).
		SetDrawingRules(q.DrawingRules).
		SetReviewRubric(q.ReviewRubric).
		SetEnabled(q.Enabled).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckQuestionNotFound, nil)
	}
	q.ID = created.ID
	q.CreatedAt = created.CreatedAt
	q.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *intelCheckRepository) GetQuestionByID(ctx context.Context, id int64) (*service.IntelCheckQuestion, error) {
	row, err := r.client.IntelCheckQuestion.Query().
		Where(intelcheckquestion.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrIntelCheckQuestionNotFound, nil)
	}
	return entToIntelCheckQuestion(row), nil
}

func (r *intelCheckRepository) UpdateQuestion(ctx context.Context, q *service.IntelCheckQuestion) error {
	client := clientFromContext(ctx, r.client)
	updated, err := client.IntelCheckQuestion.UpdateOneID(q.ID).
		SetKind(intelcheckquestion.Kind(q.Kind)).
		SetTitle(q.Title).
		SetPrompt(q.Prompt).
		SetExpectedAnswer(q.ExpectedAnswer).
		SetMatchMode(q.MatchMode).
		SetReferenceHTML(q.ReferenceHTML).
		SetReferenceMetrics(q.ReferenceMetrics).
		SetDrawingRules(q.DrawingRules).
		SetReviewRubric(q.ReviewRubric).
		SetEnabled(q.Enabled).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckQuestionNotFound, nil)
	}
	q.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *intelCheckRepository) DeleteQuestion(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.IntelCheckQuestion.DeleteOneID(id).Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckQuestionNotFound, nil)
	}
	return nil
}

func (r *intelCheckRepository) ListQuestions(
	ctx context.Context, params service.IntelCheckQuestionListParams,
) ([]*service.IntelCheckQuestion, int64, error) {
	q := r.client.IntelCheckQuestion.Query()
	if kind := strings.TrimSpace(params.Kind); kind != "" {
		q = q.Where(intelcheckquestion.KindEQ(intelcheckquestion.Kind(kind)))
	}
	if params.Enabled != nil {
		q = q.Where(intelcheckquestion.EnabledEQ(*params.Enabled))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count intel check questions: %w", err)
	}

	page, pageSize := intelCheckPaging(params.Page, params.PageSize)
	rows, err := q.
		Order(dbent.Desc(intelcheckquestion.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check questions: %w", err)
	}

	items := make([]*service.IntelCheckQuestion, 0, len(rows))
	for _, row := range rows {
		items = append(items, entToIntelCheckQuestion(row))
	}
	return items, int64(total), nil
}

// PickEnabledQuestion 取指定题型中 id 最小的启用题目。
//
// 按 id 升序而非随机：题库当前各题型只放 1 道，随机取没有意义，
// 而固定顺序能让「同一轮次内所有分组拿到同一道题」这件事显而易见——
// 分组之间的结果只有在题目相同的前提下才具备可比性。
func (r *intelCheckRepository) PickEnabledQuestion(ctx context.Context, kind string) (*service.IntelCheckQuestion, error) {
	row, err := r.client.IntelCheckQuestion.Query().
		Where(
			intelcheckquestion.KindEQ(intelcheckquestion.Kind(kind)),
			intelcheckquestion.EnabledEQ(true),
		).
		Order(dbent.Asc(intelcheckquestion.FieldID)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrIntelCheckNoQuestion
	}
	if err != nil {
		return nil, fmt.Errorf("pick enabled intel check question: %w", err)
	}
	return entToIntelCheckQuestion(row), nil
}

// ---------- ent → service 转换 ----------

func entToIntelCheckTarget(row *dbent.IntelCheckTarget) *service.IntelCheckTarget {
	if row == nil {
		return nil
	}
	return &service.IntelCheckTarget{
		ID:              row.ID,
		Name:            row.Name,
		Description:     row.Description,
		BaseURL:         row.BaseURL,
		APIKey:          row.APIKeyEncrypted, // 密文，由 service 层解密
		APIMode:         row.APIMode,
		Model:           row.Model,
		ReasoningEffort: row.ReasoningEffort,
		RateLabel:       row.RateLabel,
		Enabled:         row.Enabled,
		SortOrder:       row.SortOrder,
		CreatedBy:       row.CreatedBy,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func entToIntelCheckQuestion(row *dbent.IntelCheckQuestion) *service.IntelCheckQuestion {
	if row == nil {
		return nil
	}
	return &service.IntelCheckQuestion{
		ID:               row.ID,
		Kind:             string(row.Kind),
		Title:            row.Title,
		Prompt:           row.Prompt,
		ExpectedAnswer:   row.ExpectedAnswer,
		MatchMode:        row.MatchMode,
		ReferenceHTML:    row.ReferenceHTML,
		ReferenceMetrics: row.ReferenceMetrics,
		DrawingRules:     row.DrawingRules,
		ReviewRubric:     row.ReviewRubric,
		Enabled:          row.Enabled,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}
