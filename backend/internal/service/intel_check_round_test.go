//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProbeIntelCheckOnce_仅请求指定绘图模型且不调用评审模型(t *testing.T) {
	var requests []map[string]any
	withIntelCheckHTTPClient(t, &http.Client{Transport: intelCheckRoundTripFunc(
		func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.NewDecoder(bytes.NewReader(body)).Decode(&payload))
			requests = append(requests, payload)

			if len(requests) == 1 {
				delta, err := json.Marshal(map[string]any{
					"type": "response.output_text.delta", "delta": intelCheckPassingDrawing,
				})
				require.NoError(t, err)
				stream := "data: " + string(delta) + "\n\n" +
					`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":100,"output_tokens":200}}}` + "\n\n"
				response := intelCheckResponse(http.StatusOK, stream)
				response.Header.Set("Content-Type", "text/event-stream")
				return response, nil
			}

			return intelCheckResponse(http.StatusOK,
				`{"output":[{"type":"message","content":[{"type":"output_text","text":"{\"score\":85,\"summary\":\"通过\",\"items\":[]}"}]}]}`), nil
		},
	)})

	cfg := DefaultIntelCheckSettings()
	cfg.DrawingJudge.SkipReview = false
	cfg.DrawingJudge.Model = "judge-model"
	cfg.DrawingJudge.ReasoningEffort = IntelCheckEffortMedium
	target := &IntelCheckTarget{
		BaseURL: "https://example.test/v1", APIKey: "target-key",
		APIMode: IntelCheckAPIModeResponses, Model: "drawing-model",
		ReasoningEffort: IntelCheckEffortXHigh,
	}
	judge := &IntelCheckTarget{
		BaseURL: "https://example.test/v1", APIKey: "judge-key",
		APIMode: IntelCheckAPIModeResponses,
	}

	probe := (&IntelCheckService{}).probeIntelCheckOnce(
		context.Background(), &cfg, target, intelCheckDrawingQuestion(nil), judge,
	)

	require.Equal(t, IntelCheckStatusPass, probe.Judged.Status)
	require.Len(t, requests, 1)
	require.Equal(t, "drawing-model", requests[0]["model"])
	require.Equal(t, true, requests[0]["stream"])
	require.Equal(t, map[string]any{"effort": "xhigh"}, requests[0]["reasoning"])
	require.Equal(t, intelCheckStructureVersion, probe.Judged.JudgeDetail["judge_method"])
}

// 受检地址刻意用回环地址：safeDialContext 对 IP 字面量走快速路径，
// 命中私网段即刻返回错误，既不做 DNS 也不发包。这让「上游调用失败」这条分支
// 在离线环境下完全确定且瞬时完成，同时顺带钉死一件事——SSRF 防护确实接在了
// 智力检测的 HTTP client 上，而不是只写在设计文档里。
const intelCheckLoopbackBaseURL = "http://127.0.0.1:9"

// intelCheckBrokenCipher 解密必定失败的密文，用于构造「凭据损坏」的分组。
const intelCheckBrokenCipher = "broken-cipher"

// intelCheckEncryptorStub 恒等加解密，仅对哨兵密文报错。
type intelCheckEncryptorStub struct{}

func (intelCheckEncryptorStub) Encrypt(plain string) (string, error) { return plain, nil }

func (intelCheckEncryptorStub) Decrypt(cipher string) (string, error) {
	if cipher == intelCheckBrokenCipher {
		return "", errors.New("intel check stub: 密文损坏")
	}
	return cipher, nil
}

// intelCheckRunFixture 造一个「开关按需、一道逻辑题、一个可跑分组」的最小环境。
func intelCheckRunFixture(t *testing.T, enabled bool) (*IntelCheckService, *intelCheckRepoStub) {
	t.Helper()

	cfg := DefaultIntelCheckSettings()
	cfg.Enabled = enabled
	cfg.Concurrency = 2
	raw, err := json.Marshal(&cfg)
	require.NoError(t, err)

	repo := &intelCheckRepoStub{
		pickQuestions: map[string]*IntelCheckQuestion{
			IntelCheckKindLogic: {
				ID: 11, Kind: IntelCheckKindLogic, Prompt: "笼中鸡兔共 35 头 94 足，兔几只？",
			},
		},
		enabledTargets: []*IntelCheckTarget{{
			ID: 1, Name: "满血组", Model: "gpt-6-astra",
			ReasoningEffort: IntelCheckEffortHigh, BaseURL: intelCheckLoopbackBaseURL,
			APIKey: "sk-plain", APIMode: IntelCheckAPIModeResponses, Enabled: true,
		}},
	}

	svc := NewIntelCheckService(repo, &intelCheckSettingStoreStub{value: string(raw)}, intelCheckEncryptorStub{})
	return svc, repo
}

func TestIntelCheckRunOnce_开关关闭时不建轮次(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, false)

	round, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.ErrorIs(t, err, ErrIntelCheckDisabled)
	require.Nil(t, round)
	require.Empty(t, repo.snapshotCalls(), "关闭状态下不应留下任何写操作")
}

func TestIntelCheckRunOnce_题库为空时不建轮次(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)
	repo.pickQuestions = nil

	_, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.ErrorIs(t, err, ErrIntelCheckNoQuestion)
	// 空轮次会让公开页的轮次号凭空前进，看起来像检测跑过但全员没数据。
	require.Empty(t, repo.snapshotCalls())
}

func TestIntelCheckRunOnce_抽题的存储故障不得被当成无题(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)
	repo.pickErr = errors.New("connection refused")

	_, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	// 把连接故障静默归入「本题型无题」，整轮会悄无声息地什么都不测。
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrIntelCheckNoQuestion)
	require.Empty(t, repo.snapshotCalls())
}

func TestIntelCheckRunOnce_无可用分组仍建轮次并收尾(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)
	repo.enabledTargets[0].APIKey = intelCheckBrokenCipher

	round, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.NoError(t, err)
	require.NotNil(t, round)

	// 公开页的「下次检测时刻」锚在上一轮的开始时间上，
	// 没分组就跳过建轮，会让页面看起来调度已经停摆。
	require.Equal(t, []string{"CreateRound", "FinishRound"}, repo.snapshotCalls())
	require.Empty(t, repo.runningDrafts, "没有可跑的分组时不应落占位行")
	require.Equal(t, []int64{round.ID}, repo.finishedRounds)
}

func TestIntelCheckRunOnce_running先落占位再更新终态(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)

	round, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.NoError(t, err)

	require.Equal(t,
		[]string{"CreateRound", "CreateRunningResults", "SaveResultOutcome", "FinishRound"},
		repo.snapshotCalls(),
		"占位行必须先落库，否则一轮开始的瞬间前端看不到浅蓝色的「检测中」")

	require.Len(t, repo.runningDrafts, 1)
	require.Equal(t, round.ID, repo.runningDrafts[0].RoundID)
	require.Equal(t, IntelCheckKindLogic, repo.runningDrafts[0].Kind)
	require.Equal(t, "笼中鸡兔共 35 头 94 足，兔几只？", repo.runningDrafts[0].PromptSnapshot,
		"题面要快照：题目日后被改或被删，历史详情仍须能还原当时问了什么")

	outcomes := repo.snapshotOutcomes()
	require.Len(t, outcomes, 1, "每个（分组 × 题型）必须恰好写一次终态，漏写那一格会永远停在「检测中」")
	require.Equal(t, IntelCheckStatusRequestError, outcomes[0].Status,
		"上游不可达是链路故障，不能记成模型不合格")
}

func TestIntelCheckRunOnce_请求失败不把上游细节写进对外字段(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)

	_, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.NoError(t, err)

	outcomes := repo.snapshotOutcomes()
	require.Len(t, outcomes, 1)

	detail, err := json.Marshal(outcomes[0].JudgeDetail)
	require.NoError(t, err)
	// judge_detail 会被公开详情原样返回，上游地址只允许出现在不对外的 error_message 里。
	require.NotContains(t, string(detail), "127.0.0.1")
	require.Contains(t, outcomes[0].ErrorMessage, "127.0.0.1",
		"排障线索不能连带丢掉，它应当留在 error_message 中供管理端查看")
}

func TestIntelCheckRunOnce_两道题各落一条且坏凭据分组不牵连他组(t *testing.T) {
	svc, repo := intelCheckRunFixture(t, true)
	repo.pickQuestions[IntelCheckKindDrawing] = &IntelCheckQuestion{
		ID: 22, Kind: IntelCheckKindDrawing, Prompt: "画一只鹈鹕骑自行车",
	}
	repo.enabledTargets = append(repo.enabledTargets, &IntelCheckTarget{
		ID: 2, Name: "坏凭据组", Model: "gpt-6-astra",
		ReasoningEffort: IntelCheckEffortHigh, BaseURL: intelCheckLoopbackBaseURL,
		APIKey: intelCheckBrokenCipher, APIMode: IntelCheckAPIModeResponses, Enabled: true,
	})

	round, err := svc.RunOnce(context.Background(), IntelCheckTriggerCron)
	require.NoError(t, err)

	require.Len(t, repo.runningDrafts, 2, "只有 1 号分组可跑，两道题各占一行")

	outcomes := repo.snapshotOutcomes()
	require.Len(t, outcomes, 2)
	require.ElementsMatch(t,
		[]string{IntelCheckKindLogic, IntelCheckKindDrawing},
		[]string{outcomes[0].Kind, outcomes[1].Kind})
	for _, outcome := range outcomes {
		require.Equal(t, round.ID, outcome.RoundID)
		require.EqualValues(t, 1, outcome.TargetID,
			"一把坏密钥只让该分组缺席本轮，不应牵连其他分组")
	}
}
