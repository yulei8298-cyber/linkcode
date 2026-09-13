package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/intelcheckresult"
	"github.com/Wei-Shaw/sub2api/ent/intelcheckround"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// ---------- 轮次 ----------

// CreateRound 先向序列取号再插入。
//
// 事务回滚时已取的号不会归还，轮次号因此可能出现空洞。这是有意接受的：
// 空洞只影响展示连续性，而复用号码会撞唯一索引导致整轮检测失败。
func (r *intelCheckRepository) CreateRound(ctx context.Context, round *service.IntelCheckRound) error {
	var seq int64
	if err := r.db.QueryRowContext(ctx, `SELECT nextval('intel_check_rounds_seq_seq')`).Scan(&seq); err != nil {
		return fmt.Errorf("allocate intel check round seq: %w", err)
	}

	client := clientFromContext(ctx, r.client)
	builder := client.IntelCheckRound.Create().
		SetSeq(seq).
		SetStartedAt(round.StartedAt).
		SetTriggerSource(round.TriggerSource).
		SetNillableLogicQuestionID(round.LogicQuestionID).
		SetNillableDrawingQuestionID(round.DrawingQuestionID)

	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckRoundNotFound, nil)
	}
	round.ID = created.ID
	round.Seq = created.Seq
	round.StartedAt = created.StartedAt
	return nil
}

func (r *intelCheckRepository) FinishRound(ctx context.Context, id int64, finishedAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	if err := client.IntelCheckRound.UpdateOneID(id).
		SetFinishedAt(finishedAt).
		Exec(ctx); err != nil {
		return translatePersistenceError(err, service.ErrIntelCheckRoundNotFound, nil)
	}
	return nil
}

func (r *intelCheckRepository) GetRoundByID(ctx context.Context, id int64) (*service.IntelCheckRound, error) {
	row, err := r.client.IntelCheckRound.Query().
		Where(intelcheckround.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrIntelCheckRoundNotFound, nil)
	}
	return entToIntelCheckRound(row), nil
}

func (r *intelCheckRepository) ListRounds(
	ctx context.Context, params service.IntelCheckRoundListParams,
) ([]*service.IntelCheckRound, int64, error) {
	q := r.client.IntelCheckRound.Query()

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count intel check rounds: %w", err)
	}

	page, pageSize := intelCheckPaging(params.Page, params.PageSize)
	rows, err := q.
		Order(dbent.Desc(intelcheckround.FieldSeq)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check rounds: %w", err)
	}

	items := make([]*service.IntelCheckRound, 0, len(rows))
	for _, row := range rows {
		items = append(items, entToIntelCheckRound(row))
	}
	return items, int64(total), nil
}

// ---------- 明细 ----------

// CreateRunningResults 批量插入 running 占位行，冲突时不动已有行。
//
// 用 DO NOTHING 而非 UPDATE：冲突意味着该 (轮次, 分组, 题型) 已经有结果了，
// 若覆盖成 running，一次重试就会把已判定的 pass/fail 抹回「检测中」。
func (r *intelCheckRepository) CreateRunningResults(ctx context.Context, drafts []*service.IntelCheckResultDraft) error {
	if len(drafts) == 0 {
		return nil
	}
	client := clientFromContext(ctx, r.client)
	bulk := make([]*dbent.IntelCheckResultCreate, 0, len(drafts))
	for _, draft := range drafts {
		bulk = append(bulk, client.IntelCheckResult.Create().
			SetRoundID(draft.RoundID).
			SetTargetID(draft.TargetID).
			SetKind(intelcheckresult.Kind(draft.Kind)).
			SetStatus(intelcheckresult.StatusRunning).
			SetPromptSnapshot(draft.PromptSnapshot).
			SetCheckedAt(draft.CheckedAt))
	}

	if err := client.IntelCheckResult.CreateBulk(bulk...).
		OnConflictColumns(
			intelcheckresult.FieldRoundID,
			intelcheckresult.FieldTargetID,
			intelcheckresult.FieldKind,
		).
		Ignore().
		Exec(ctx); err != nil {
		return fmt.Errorf("create running intel check results: %w", err)
	}
	return nil
}

// SaveResultOutcome 把占位行原地更新为最终结果。
//
// 按 (round_id, target_id, kind) 而非主键定位：调用方是执行器，
// 它手上只有这三个维度，不必为了拿 id 再查一次库。
func (r *intelCheckRepository) SaveResultOutcome(ctx context.Context, outcome *service.IntelCheckResultOutcome) error {
	client := clientFromContext(ctx, r.client)
	affected, err := client.IntelCheckResult.Update().
		Where(
			intelcheckresult.RoundIDEQ(outcome.RoundID),
			intelcheckresult.TargetIDEQ(outcome.TargetID),
			intelcheckresult.KindEQ(intelcheckresult.Kind(outcome.Kind)),
		).
		SetStatus(intelcheckresult.Status(outcome.Status)).
		SetNillableLatencyMs(outcome.LatencyMs).
		SetRawReply(outcome.RawReply).
		SetExtractedAnswer(outcome.ExtractedAnswer).
		SetHTMLOutput(outcome.HTMLOutput).
		SetJudgeDetail(outcome.JudgeDetail).
		SetErrorMessage(outcome.ErrorMessage).
		SetNillableInputTokens(outcome.InputTokens).
		SetNillableOutputTokens(outcome.OutputTokens).
		SetCheckedAt(outcome.CheckedAt).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("save intel check result outcome: %w", err)
	}
	if affected == 0 {
		return service.ErrIntelCheckResultNotFound
	}
	return nil
}

func (r *intelCheckRepository) GetResultByID(ctx context.Context, id int64) (*service.IntelCheckResult, error) {
	row, err := r.client.IntelCheckResult.Query().
		Where(intelcheckresult.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrIntelCheckResultNotFound, nil)
	}
	return entToIntelCheckResult(row), nil
}

func (r *intelCheckRepository) ListResults(
	ctx context.Context, params service.IntelCheckResultListParams,
) ([]*service.IntelCheckResult, int64, error) {
	q := r.client.IntelCheckResult.Query()
	if params.RoundID != nil {
		q = q.Where(intelcheckresult.RoundIDEQ(*params.RoundID))
	}
	if params.TargetID != nil {
		q = q.Where(intelcheckresult.TargetIDEQ(*params.TargetID))
	}
	if params.Kind != "" {
		q = q.Where(intelcheckresult.KindEQ(intelcheckresult.Kind(params.Kind)))
	}
	if params.Status != "" {
		q = q.Where(intelcheckresult.StatusEQ(intelcheckresult.Status(params.Status)))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count intel check results: %w", err)
	}

	page, pageSize := intelCheckPaging(params.Page, params.PageSize)
	rows, err := q.
		Order(dbent.Desc(intelcheckresult.FieldCheckedAt), dbent.Desc(intelcheckresult.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check results: %w", err)
	}

	items := make([]*service.IntelCheckResult, 0, len(rows))
	for _, row := range rows {
		items = append(items, entToIntelCheckResult(row))
	}
	return items, int64(total), nil
}

// ---------- 公开页聚合 ----------

// timelineQuery 每个 (分组, 题型) 各取最近 limit 条。
//
// 走窗口函数而非「循环每组查一次」：分组数 × 题型数 会放大成几十次往返。
// 只 SELECT 色块需要的 5 列，绝不带 raw_reply / html_output。
const timelineQuery = `
SELECT id, target_id, kind, status, latency_ms, checked_at
FROM (
    SELECT id, target_id, kind, status, latency_ms, checked_at,
           ROW_NUMBER() OVER (
               PARTITION BY target_id, kind
               ORDER BY checked_at DESC, id DESC
           ) AS rn
    FROM intel_check_results
    WHERE target_id = ANY($1)
) ranked
WHERE rn <= $2
ORDER BY target_id, kind, checked_at ASC, id ASC`

func (r *intelCheckRepository) ListTimelineForTargets(
	ctx context.Context, targetIDs []int64, limit int,
) (map[int64]map[string][]*service.IntelCheckTimelinePoint, error) {
	out := make(map[int64]map[string][]*service.IntelCheckTimelinePoint)
	if len(targetIDs) == 0 || limit <= 0 {
		return out, nil
	}

	rows, err := r.db.QueryContext(ctx, timelineQuery, pq.Array(targetIDs), limit)
	if err != nil {
		return nil, fmt.Errorf("query intel check timeline: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			point    service.IntelCheckTimelinePoint
			targetID int64
			kind     string
		)
		if err := rows.Scan(
			&point.ResultID, &targetID, &kind,
			&point.Status, &point.LatencyMs, &point.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan intel check timeline: %w", err)
		}
		if out[targetID] == nil {
			out[targetID] = make(map[string][]*service.IntelCheckTimelinePoint)
		}
		out[targetID][kind] = append(out[targetID][kind], &point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate intel check timeline: %w", err)
	}
	return out, nil
}

// latestDrawingQuery 每组取最近一条「确实有画作」的明细。
//
// 条件取「html_output 非空」而非「status 为 pass」：判失败的画作同样要展示，
// 页面要让用户看到降智时画成了什么样，这正是本功能的说服力所在。
//
// 注：此处刻意不写出那个由两个单引号组成的空串字面量。gofmt 会重排文档注释，
// 并沿用 godoc 的老式引号约定把它转成一个右双引号，静默改坏注释语义。
const latestDrawingQuery = `
SELECT DISTINCT ON (target_id) target_id, id
FROM intel_check_results
WHERE target_id = ANY($1) AND kind = 'drawing' AND html_output <> ''
ORDER BY target_id, checked_at DESC, id DESC`

func (r *intelCheckRepository) LatestDrawingResultIDs(
	ctx context.Context, targetIDs []int64,
) (map[int64]int64, error) {
	out := make(map[int64]int64)
	if len(targetIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.QueryContext(ctx, latestDrawingQuery, pq.Array(targetIDs))
	if err != nil {
		return nil, fmt.Errorf("query latest intel check drawings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var targetID, resultID int64
		if err := rows.Scan(&targetID, &resultID); err != nil {
			return nil, fmt.Errorf("scan latest intel check drawings: %w", err)
		}
		out[targetID] = resultID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate latest intel check drawings: %w", err)
	}
	return out, nil
}

const countByStatusQuery = `
SELECT target_id, status, COUNT(*)
FROM intel_check_results
WHERE target_id = ANY($1) AND kind = $2 AND checked_at >= $3
GROUP BY target_id, status`

func (r *intelCheckRepository) CountByStatusSince(
	ctx context.Context, targetIDs []int64, kind string, since time.Time,
) (map[int64]map[string]int64, error) {
	out := make(map[int64]map[string]int64)
	if len(targetIDs) == 0 {
		return out, nil
	}

	rows, err := r.db.QueryContext(ctx, countByStatusQuery, pq.Array(targetIDs), kind, since)
	if err != nil {
		return nil, fmt.Errorf("count intel check results by status: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var (
			targetID int64
			status   string
			count    int64
		)
		if err := rows.Scan(&targetID, &status, &count); err != nil {
			return nil, fmt.Errorf("scan intel check status counts: %w", err)
		}
		if out[targetID] == nil {
			out[targetID] = make(map[string]int64)
		}
		out[targetID][status] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate intel check status counts: %w", err)
	}
	return out, nil
}

// ---------- 留存清理 ----------

func (r *intelCheckRepository) DeleteResultsBefore(ctx context.Context, before time.Time) (int64, error) {
	deleted, err := r.client.IntelCheckResult.Delete().
		Where(intelcheckresult.CheckedAtLT(before)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired intel check results: %w", err)
	}
	return int64(deleted), nil
}

// deleteEmptyRoundsQuery 只删已经没有明细的轮次。
//
// 不能直接按 started_at 删轮次：外键是 ON DELETE CASCADE，
// 那会把尚在留存期内的明细一起带走。
const deleteEmptyRoundsQuery = `
DELETE FROM intel_check_rounds r
WHERE r.started_at < $1
  AND NOT EXISTS (SELECT 1 FROM intel_check_results x WHERE x.round_id = r.id)`

func (r *intelCheckRepository) DeleteEmptyRoundsBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, deleteEmptyRoundsQuery, before)
	if err != nil {
		return 0, fmt.Errorf("delete empty intel check rounds: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted intel check rounds count: %w", err)
	}
	return affected, nil
}

// ---------- ent → service 转换 ----------

func entToIntelCheckRound(row *dbent.IntelCheckRound) *service.IntelCheckRound {
	if row == nil {
		return nil
	}
	return &service.IntelCheckRound{
		ID:                row.ID,
		Seq:               row.Seq,
		StartedAt:         row.StartedAt,
		FinishedAt:        row.FinishedAt,
		LogicQuestionID:   row.LogicQuestionID,
		DrawingQuestionID: row.DrawingQuestionID,
		TriggerSource:     row.TriggerSource,
	}
}

func entToIntelCheckResult(row *dbent.IntelCheckResult) *service.IntelCheckResult {
	if row == nil {
		return nil
	}
	return &service.IntelCheckResult{
		ID:              row.ID,
		RoundID:         row.RoundID,
		TargetID:        row.TargetID,
		Kind:            string(row.Kind),
		Status:          string(row.Status),
		LatencyMs:       row.LatencyMs,
		PromptSnapshot:  row.PromptSnapshot,
		RawReply:        row.RawReply,
		ExtractedAnswer: row.ExtractedAnswer,
		HTMLOutput:      row.HTMLOutput,
		JudgeDetail:     row.JudgeDetail,
		ErrorMessage:    row.ErrorMessage,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		CheckedAt:       row.CheckedAt,
	}
}
