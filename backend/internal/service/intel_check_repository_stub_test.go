//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// 本文件只放 IntelCheckRepository 与设置存储的测试替身，不含用例。
//
// 单独成文是因为接口有 25 个方法，而用到它的地方不止一处（公开页聚合、
// 后续的 runner、CRUD）。若每个用例文件各写一份，接口增删一个方法就要改好几处，
// 漏改的那份会编译失败但报错位置离真正的改动很远。

// errIntelCheckStubUnsupported 未被当前用例桩接的方法。
//
// 返回错误而非 panic：真正的误用会在断言里以「err 非 nil」的形式暴露，
// 而 panic 会把整个测试二进制打断，看不出是哪个用例走错了路径。
var errIntelCheckStubUnsupported = errors.New("intel check repo stub: 该方法未在本用例中桩接")

// intelCheckRepoStub 按字段桩接返回值，零值即「无数据」。
type intelCheckRepoStub struct {
	enabledTargets []*IntelCheckTarget
	targets        map[int64]*IntelCheckTarget
	questions      map[int64]*IntelCheckQuestion
	rounds         []*IntelCheckRound
	roundsByID     map[int64]*IntelCheckRound
	results        map[int64]*IntelCheckResult

	timelines map[int64]map[string][]*IntelCheckTimelinePoint
	// counts 按 kind → targetID → status → 条数 组织，模拟 CountByStatusSince 的按题型过滤。
	counts   map[string]map[int64]map[string]int64
	artworks map[int64]int64

	// countKinds 记录 CountByStatusSince 实际收到的 kind 入参。
	// 该参数会直接落到 SQL 的枚举列上，传空串是非法值而非「不过滤」，必须可断言。
	countKinds []string
	// countSince 记录最后一次统计的时间窗起点。
	countSince time.Time

	// listEnabledErr 等错误注入字段，用于验证失败路径不被吞掉。
	listEnabledErr error
	timelineErr    error
	countErr       error
	roundsErr      error

	// ---------- 以下字段服务于轮次执行（runner）用例 ----------

	// pickQuestions 按 kind 桩接抽题结果；缺项即该题型无题。
	pickQuestions map[string]*IntelCheckQuestion
	pickErr       error

	createRoundErr error
	runningErr     error
	finishErr      error
	outcomeErr     error

	// mu 保护以下字段：一轮里每个分组占一个 worker，SaveResultOutcome 会被并发调用，
	// 不加锁在 -race 下必然报数据竞争，而那种失败看起来像是被测代码的问题。
	mu             sync.Mutex
	lastRoundID    int64
	finishedRounds []int64
	runningDrafts  []*IntelCheckResultDraft
	outcomes       []*IntelCheckResultOutcome
	// callOrder 记录写操作的先后，用于断言「running 先写、终态后更」。
	callOrder []string
}

func cloneIntelCheckTarget(target *IntelCheckTarget) *IntelCheckTarget {
	if target == nil {
		return nil
	}
	clone := *target
	return &clone
}

func cloneIntelCheckQuestion(question *IntelCheckQuestion) *IntelCheckQuestion {
	if question == nil {
		return nil
	}
	clone := *question
	if question.ReferenceMetrics != nil {
		clone.ReferenceMetrics = make(map[string]any, len(question.ReferenceMetrics))
		for key, value := range question.ReferenceMetrics {
			clone.ReferenceMetrics[key] = value
		}
	}
	if question.DrawingRules != nil {
		clone.DrawingRules = make(map[string]any, len(question.DrawingRules))
		for key, value := range question.DrawingRules {
			clone.DrawingRules[key] = value
		}
	}
	return &clone
}

// record 记录一次写操作。
func (s *intelCheckRepoStub) record(op string) {
	s.mu.Lock()
	s.callOrder = append(s.callOrder, op)
	s.mu.Unlock()
}

// snapshotCalls 取调用顺序副本，避免用例直接读被并发写的切片。
func (s *intelCheckRepoStub) snapshotCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.callOrder...)
}

// snapshotOutcomes 取终态结果副本。
func (s *intelCheckRepoStub) snapshotOutcomes() []*IntelCheckResultOutcome {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*IntelCheckResultOutcome(nil), s.outcomes...)
}

// ---------- 受检分组 ----------

func (s *intelCheckRepoStub) CreateTarget(_ context.Context, target *IntelCheckTarget) error {
	if s.targets == nil {
		s.targets = make(map[int64]*IntelCheckTarget)
	}
	if target.ID <= 0 {
		target.ID = int64(len(s.targets) + 1)
	}
	s.targets[target.ID] = cloneIntelCheckTarget(target)
	return nil
}

func (s *intelCheckRepoStub) GetTargetByID(_ context.Context, id int64) (*IntelCheckTarget, error) {
	if target, ok := s.targets[id]; ok {
		return cloneIntelCheckTarget(target), nil
	}
	return nil, ErrIntelCheckTargetNotFound
}

func (s *intelCheckRepoStub) UpdateTarget(_ context.Context, target *IntelCheckTarget) error {
	if target == nil || target.ID <= 0 {
		return ErrIntelCheckTargetNotFound
	}
	if s.targets == nil {
		return ErrIntelCheckTargetNotFound
	}
	if _, ok := s.targets[target.ID]; !ok {
		return ErrIntelCheckTargetNotFound
	}
	s.targets[target.ID] = cloneIntelCheckTarget(target)
	return nil
}

func (s *intelCheckRepoStub) DeleteTarget(_ context.Context, id int64) error {
	if s.targets == nil {
		return ErrIntelCheckTargetNotFound
	}
	if _, ok := s.targets[id]; !ok {
		return ErrIntelCheckTargetNotFound
	}
	delete(s.targets, id)
	return nil
}

func (s *intelCheckRepoStub) ListTargets(
	context.Context, IntelCheckTargetListParams,
) ([]*IntelCheckTarget, int64, error) {
	return nil, 0, errIntelCheckStubUnsupported
}

func (s *intelCheckRepoStub) ListEnabledTargets(context.Context) ([]*IntelCheckTarget, error) {
	if s.listEnabledErr != nil {
		return nil, s.listEnabledErr
	}
	return s.enabledTargets, nil
}

// ---------- 题库 ----------

func (s *intelCheckRepoStub) CreateQuestion(_ context.Context, question *IntelCheckQuestion) error {
	if s.questions == nil {
		s.questions = make(map[int64]*IntelCheckQuestion)
	}
	if question.ID <= 0 {
		question.ID = int64(len(s.questions) + 1)
	}
	s.questions[question.ID] = cloneIntelCheckQuestion(question)
	return nil
}

func (s *intelCheckRepoStub) GetQuestionByID(_ context.Context, id int64) (*IntelCheckQuestion, error) {
	if question, ok := s.questions[id]; ok {
		return cloneIntelCheckQuestion(question), nil
	}
	return nil, ErrIntelCheckQuestionNotFound
}

func (s *intelCheckRepoStub) UpdateQuestion(_ context.Context, question *IntelCheckQuestion) error {
	if question == nil || question.ID <= 0 {
		return ErrIntelCheckQuestionNotFound
	}
	if s.questions == nil {
		return ErrIntelCheckQuestionNotFound
	}
	if _, ok := s.questions[question.ID]; !ok {
		return ErrIntelCheckQuestionNotFound
	}
	s.questions[question.ID] = cloneIntelCheckQuestion(question)
	return nil
}

func (s *intelCheckRepoStub) DeleteQuestion(_ context.Context, id int64) error {
	if s.questions == nil {
		return ErrIntelCheckQuestionNotFound
	}
	if _, ok := s.questions[id]; !ok {
		return ErrIntelCheckQuestionNotFound
	}
	delete(s.questions, id)
	return nil
}

func (s *intelCheckRepoStub) ListQuestions(
	context.Context, IntelCheckQuestionListParams,
) ([]*IntelCheckQuestion, int64, error) {
	return nil, 0, errIntelCheckStubUnsupported
}

func (s *intelCheckRepoStub) PickEnabledQuestion(_ context.Context, kind string) (*IntelCheckQuestion, error) {
	if s.pickErr != nil {
		return nil, s.pickErr
	}
	if question, ok := s.pickQuestions[kind]; ok && question != nil {
		return question, nil
	}
	return nil, ErrIntelCheckNoQuestion
}

// ---------- 轮次 ----------

// CreateRound 模拟真实仓储：取号并就地回填 ID 与 Seq。
// 回填这一步必须留在桩里——runner 刻意不自己设 Seq（重复赋值会撞唯一索引），
// 桩若不回填，后续所有以 round.ID 为键的断言都会退化成对 0 的断言。
func (s *intelCheckRepoStub) CreateRound(_ context.Context, round *IntelCheckRound) error {
	s.record("CreateRound")
	if s.createRoundErr != nil {
		return s.createRoundErr
	}
	s.mu.Lock()
	s.lastRoundID++
	round.ID = s.lastRoundID
	round.Seq = s.lastRoundID
	s.mu.Unlock()
	if round.StartedAt.IsZero() {
		round.StartedAt = time.Now()
	}
	return nil
}

func (s *intelCheckRepoStub) FinishRound(_ context.Context, roundID int64, _ time.Time) error {
	s.record("FinishRound")
	if s.finishErr != nil {
		return s.finishErr
	}
	s.mu.Lock()
	s.finishedRounds = append(s.finishedRounds, roundID)
	s.mu.Unlock()
	return nil
}

func (s *intelCheckRepoStub) GetRoundByID(_ context.Context, id int64) (*IntelCheckRound, error) {
	if round, ok := s.roundsByID[id]; ok {
		return round, nil
	}
	return nil, ErrIntelCheckRoundNotFound
}

func (s *intelCheckRepoStub) ListRounds(
	_ context.Context, params IntelCheckRoundListParams,
) ([]*IntelCheckRound, int64, error) {
	if s.roundsErr != nil {
		return nil, 0, s.roundsErr
	}
	// 真实实现按 seq 降序分页，这里只需保证「取 PageSize 条」这一点一致，
	// 由用例负责把 rounds 按降序摆好。
	if params.PageSize > 0 && len(s.rounds) > params.PageSize {
		return s.rounds[:params.PageSize], int64(len(s.rounds)), nil
	}
	return s.rounds, int64(len(s.rounds)), nil
}

// ---------- 明细 ----------

func (s *intelCheckRepoStub) CreateRunningResults(_ context.Context, drafts []*IntelCheckResultDraft) error {
	s.record("CreateRunningResults")
	if s.runningErr != nil {
		return s.runningErr
	}
	s.mu.Lock()
	s.runningDrafts = append(s.runningDrafts, drafts...)
	s.mu.Unlock()
	return nil
}

func (s *intelCheckRepoStub) SaveResultOutcome(_ context.Context, outcome *IntelCheckResultOutcome) error {
	s.record("SaveResultOutcome")
	if s.outcomeErr != nil {
		return s.outcomeErr
	}
	s.mu.Lock()
	s.outcomes = append(s.outcomes, outcome)
	s.mu.Unlock()
	return nil
}

func (s *intelCheckRepoStub) GetResultByID(_ context.Context, id int64) (*IntelCheckResult, error) {
	if result, ok := s.results[id]; ok {
		return result, nil
	}
	return nil, ErrIntelCheckResultNotFound
}

func (s *intelCheckRepoStub) ListResults(
	context.Context, IntelCheckResultListParams,
) ([]*IntelCheckResult, int64, error) {
	return nil, 0, errIntelCheckStubUnsupported
}

// ---------- 公开页聚合 ----------

func (s *intelCheckRepoStub) ListTimelineForTargets(
	_ context.Context, _ []int64, _ int,
) (map[int64]map[string][]*IntelCheckTimelinePoint, error) {
	if s.timelineErr != nil {
		return nil, s.timelineErr
	}
	return s.timelines, nil
}

func (s *intelCheckRepoStub) LatestDrawingResultIDs(
	context.Context, []int64,
) (map[int64]int64, error) {
	return s.artworks, nil
}

func (s *intelCheckRepoStub) CountByStatusSince(
	_ context.Context, _ []int64, kind string, since time.Time,
) (map[int64]map[string]int64, error) {
	s.countKinds = append(s.countKinds, kind)
	s.countSince = since
	if s.countErr != nil {
		return nil, s.countErr
	}
	return s.counts[kind], nil
}

// ---------- 留存清理 ----------

func (s *intelCheckRepoStub) DeleteResultsBefore(context.Context, time.Time) (int64, error) {
	return 0, errIntelCheckStubUnsupported
}

func (s *intelCheckRepoStub) DeleteEmptyRoundsBefore(context.Context, time.Time) (int64, error) {
	return 0, errIntelCheckStubUnsupported
}

// intelCheckSettingStoreStub 桩接 settings 表。
// value 为空串时返回 ErrSettingNotFound，走「首次读取回落默认值」那条路径。
type intelCheckSettingStoreStub struct {
	value  string
	getErr error
	saved  string
}

func (s *intelCheckSettingStoreStub) GetValue(_ context.Context, _ string) (string, error) {
	if s.getErr != nil {
		return "", s.getErr
	}
	if s.value == "" {
		return "", ErrSettingNotFound
	}
	return s.value, nil
}

func (s *intelCheckSettingStoreStub) Set(_ context.Context, _ string, value string) error {
	s.saved = value
	return nil
}
