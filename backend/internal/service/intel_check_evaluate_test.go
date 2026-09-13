//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// 本文件验的是判定编排层的**方向**：哪一种失败算模型不合格（红块），
// 哪一种算链路故障（黄块）。这条界线判反的后果是不对称的——
// 把自家网络抖动渲染成「这个分组降智了」，正好是本功能对外承诺的反面，
// 而且错了不会有人来报，只会被截图传播。
//
// 各判定纯函数自身的行为由 logic / gate / drawing / review 几个测试文件覆盖，
// 这里只钉编排：谁在什么条件下被调用、失败如何归类、什么能对外露出。

// intelCheckPassingDrawing 能通过结构门禁固定项的最小产物：
// 可解析 SVG + title/desc + 造型 + 一种动画机制。参考指标留空时相对项全部跳过。
const intelCheckPassingDrawing = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120">
  <title>鹈鹕骑行</title>
  <desc>一只鹈鹕骑着自行车匀速前进</desc>
  <circle cx="40" cy="90" r="18" fill="none" stroke="#111" stroke-width="3">
    <animateTransform attributeName="transform" type="rotate"
      from="0 40 90" to="360 40 90" dur="2s" repeatCount="indefinite"/>
  </circle>
  <path d="M20 60 L60 40 L100 60" fill="none" stroke="#111" stroke-width="3"/>
</svg>`

// intelCheckGatelessDrawing 缺 title/desc，固定项必挂，用于验证门禁短路。
const intelCheckGatelessDrawing = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120">
  <circle cx="40" cy="90" r="18" fill="#111"/>
</svg>`

// intelCheckDetailJSON 把对外明细序列化，供泄露断言使用。
func intelCheckDetailJSON(t *testing.T, outcome intelCheckJudgeOutcome) string {
	t.Helper()
	raw, err := json.Marshal(outcome.JudgeDetail)
	require.NoError(t, err)
	return string(raw)
}

// intelCheckDrawingQuestion 造一道绘图题，mutate 用于逐条破坏字段。
func intelCheckDrawingQuestion(mutate func(q *IntelCheckQuestion)) *IntelCheckQuestion {
	question := &IntelCheckQuestion{
		ID:     22,
		Kind:   IntelCheckKindDrawing,
		Prompt: "画一只鹈鹕骑自行车，要有持续动画",
	}
	if mutate != nil {
		mutate(question)
	}
	return question
}

// ---------- 逻辑题 ----------

func TestJudgeIntelCheckLogic_答对记通过并留下可复核明细(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "23", MatchMode: IntelCheckMatchExact,
	}

	outcome := judgeIntelCheckLogic(question, "先设兔为 x……\n最终答案：23")

	require.Equal(t, IntelCheckStatusPass, outcome.Status)
	require.Equal(t, "23", outcome.ExtractedAnswer)
	require.Empty(t, outcome.ErrorMessage)
	// 四个字段缺一不可：公开页要让读者自己核对「期望什么、提取到什么、怎么比的」，
	// 少一项判定就从可复核退化成「相信我们」。
	require.Equal(t, IntelCheckMatchExact, outcome.JudgeDetail["match_mode"])
	require.Equal(t, "23", outcome.JudgeDetail["expected_answer"])
	require.Equal(t, "23", outcome.JudgeDetail["extracted_answer"])
	require.Equal(t, true, outcome.JudgeDetail["matched"])
}

func TestJudgeIntelCheckLogic_答错记失败(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "23", MatchMode: IntelCheckMatchExact,
	}

	outcome := judgeIntelCheckLogic(question, "最终答案：24")

	require.Equal(t, IntelCheckStatusFail, outcome.Status)
	require.Equal(t, "24", outcome.ExtractedAnswer)
	require.Equal(t, false, outcome.JudgeDetail["matched"])
}

func TestJudgeIntelCheckLogic_非法正则记请求失败而非失败(t *testing.T) {
	question := &IntelCheckQuestion{
		Kind: IntelCheckKindLogic, ExpectedAnswer: "答案[", MatchMode: IntelCheckMatchRegex,
	}

	outcome := judgeIntelCheckLogic(question, "最终答案：23")

	// 管理端填错正则是题目配置问题。算成 fail 会在所有分组的时间线上
	// 同时刷出红块，看起来像全网降智，实际只是一个括号没闭合。
	require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
	require.Contains(t, outcome.ErrorMessage, "正则")
	require.Equal(t, "题目的匹配规则无法执行，本次不计入判定", outcome.JudgeDetail["reason"])
	require.NotContains(t, intelCheckDetailJSON(t, outcome), "error parsing regexp",
		"正则引擎的报错原文属于内部细节，不进对外明细")
}

func TestJudgeIntelCheckLogic_题目未配期望答案记请求失败(t *testing.T) {
	question := &IntelCheckQuestion{Kind: IntelCheckKindLogic, MatchMode: IntelCheckMatchExact}

	outcome := judgeIntelCheckLogic(question, "最终答案：23")

	// 同上：没填答案的题目谁都答不对，不能把它记成模型的锅。
	require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
}

// ---------- 绘图题：判定方向 ----------

func TestJudgeIntelCheckDrawing_门禁规则损坏记请求失败(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()
	question := intelCheckDrawingQuestion(func(q *IntelCheckQuestion) {
		q.DrawingRules = map[string]any{"min_ratio": "不是数字"}
	})

	outcome := svc.judgeIntelCheckDrawing(context.Background(), &cfg, question, nil, intelCheckPassingDrawing)

	require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
	require.Equal(t, "题目的绘图判定规则无法解析，本次不计入判定", outcome.JudgeDetail["reason"])
	require.Empty(t, outcome.HTMLOutput, "规则都没读出来，谈不上有可展示的产物")
}

func TestJudgeIntelCheckDrawing_回复里没有产物记失败(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()

	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, intelCheckDrawingQuestion(nil), nil, "抱歉，我没法画图。")

	// 答非所问是实打实的能力缺失，而不是链路故障——这是少数几条
	// 「模型自己没做到」的红块来源之一。
	require.Equal(t, IntelCheckStatusFail, outcome.Status)
	require.Equal(t, false, outcome.JudgeDetail["gate_pass"])
	require.Equal(t, "回复中没有可提取的 HTML 或 SVG 产物", outcome.JudgeDetail["reason"])
}

func TestJudgeIntelCheckDrawing_产物超体积上限记失败并说明上限(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()
	question := intelCheckDrawingQuestion(func(q *IntelCheckQuestion) {
		q.DrawingRules = map[string]any{"max_bytes": 64}
	})

	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, question, nil, intelCheckPassingDrawing)

	require.Equal(t, IntelCheckStatusFail, outcome.Status)
	require.Equal(t, "产物体积超过 64 字节上限", outcome.JudgeDetail["reason"],
		"理由要带上限数字，否则管理员无从判断是模型话多还是阈值设窄了")
}

func TestJudgeIntelCheckDrawing_产物中没有SVG记失败但保留产物(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()
	reply := "```html\n<html><body><p>这里应该有张图</p></body></html>\n```"

	outcome := svc.judgeIntelCheckDrawing(context.Background(), &cfg, intelCheckDrawingQuestion(nil), nil, reply)

	require.Equal(t, IntelCheckStatusFail, outcome.Status)
	require.Equal(t, "产物中没有可解析的 SVG 内容", outcome.JudgeDetail["reason"])
	require.NotEmpty(t, outcome.HTMLOutput,
		"产物已清洗完毕，要留给详情页展示——判失败也得让人看见失败在哪")
}

func TestJudgeIntelCheckDrawing_参考指标损坏记请求失败(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()
	question := intelCheckDrawingQuestion(func(q *IntelCheckQuestion) {
		q.ReferenceMetrics = map[string]any{"shape_count": "很多"}
	})

	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, question, nil, intelCheckPassingDrawing)

	// 没有标尺就没法比对，此时判失败等于凭空冤枉；只能记成本次不计入。
	require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
	require.Equal(t, "题目的参考稿指标无法解析，本次不计入判定", outcome.JudgeDetail["reason"])
}

func TestJudgeIntelCheckDrawing_门禁未过时不调用评审模型(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()

	// judge 传 nil：一旦进了评审分支，reviewIntelCheckDrawing 立刻报
	// 「评审分组不可用」并转成 request_error。因此这里断言 fail，
	// 等价于断言「门禁没过就压根没走评审」——既省 token，
	// 也避免评审模型对着残缺产物打出高分。
	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, intelCheckDrawingQuestion(nil), nil, intelCheckGatelessDrawing)

	require.Equal(t, IntelCheckStatusFail, outcome.Status)
	require.Equal(t, false, outcome.JudgeDetail["gate_pass"])
	require.Equal(t, "结构门禁未通过，未进入源码评审", outcome.JudgeDetail["reason"])
	require.NotEmpty(t, outcome.JudgeDetail["gate_items"], "逐条门禁结果要落进明细供读者自行复核")
}

func TestJudgeIntelCheckDrawing_评审分组缺失记请求失败(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()

	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, intelCheckDrawingQuestion(nil), nil, intelCheckPassingDrawing)

	// 评审分组没配、被删或凭据解不开，责任都在我们这侧，不能算受检模型不合格。
	require.Equal(t, IntelCheckStatusRequestError, outcome.Status)
	require.Equal(t, "评审链路未能完成，本次不计入判定", outcome.JudgeDetail["reason"])
	require.Contains(t, outcome.ErrorMessage, "评审分组不可用")
	require.NotEmpty(t, outcome.HTMLOutput, "产物本身是好的，仍要留下来展示")
}

func TestJudgeIntelCheckDrawing_评审上游不可达记请求失败且不泄露地址(t *testing.T) {
	svc := &IntelCheckService{}
	cfg := DefaultIntelCheckSettings()
	cfg.DrawingJudge.Model = "gpt-6-astra"
	judge := &IntelCheckTarget{
		ID: 9, Name: "评审组", BaseURL: intelCheckLoopbackBaseURL,
		APIKey: "sk-judge", APIMode: IntelCheckAPIModeResponses,
	}

	outcome := svc.judgeIntelCheckDrawing(
		context.Background(), &cfg, intelCheckDrawingQuestion(nil), judge, intelCheckPassingDrawing)

	require.Equal(t, IntelCheckStatusRequestError, outcome.Status,
		"评审侧的抖动不得在受检分组的时间线上留下红块")

	// judge_detail 会被公开详情原样返回；评审分组的地址属于内部拓扑。
	require.NotContains(t, intelCheckDetailJSON(t, outcome), "127.0.0.1")
	require.Contains(t, outcome.ErrorMessage, "评审模型调用失败")
}

func TestJudgeIntelCheckDrawing_两层皆过才算通过(t *testing.T) {
	gate := IntelCheckGateResult{Pass: true, Items: []IntelCheckGateItem{{Item: "包含可解析的 SVG", Pass: true}}}

	status, detail := DecideIntelCheckDrawing(gate, &IntelCheckReviewResult{Score: 80, Summary: "接近参考稿"}, 80)
	require.Equal(t, IntelCheckStatusPass, status, "恰好压线应判通过")
	require.Equal(t, 80, detail["review_score"])

	status, _ = DecideIntelCheckDrawing(gate, &IntelCheckReviewResult{Score: 79}, 80)
	require.Equal(t, IntelCheckStatusFail, status, "差一分即不合格")

	// 门禁过了却拿不到评审结果，是编排层出了岔子（正常路径下不会发生）。
	// 此时宁可判失败也不能悄悄判通过——静默放行会让整个公开页失去意义。
	status, detail = DecideIntelCheckDrawing(gate, nil, 80)
	require.Equal(t, IntelCheckStatusFail, status)
	require.Equal(t, "结构门禁通过，但未获得评审结果", detail["reason"])
}
