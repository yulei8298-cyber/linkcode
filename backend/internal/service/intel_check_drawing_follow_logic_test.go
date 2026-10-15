//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func roundPoint(resultID, roundID int64, status string) *IntelCheckTimelinePoint {
	return &IntelCheckTimelinePoint{ResultID: resultID, RoundID: roundID, Status: status, CheckedAt: time.Now()}
}

func statusesOf(points []*IntelCheckTimelinePoint) []string {
	return intelCheckTimelineStatuses(points)
}

func TestFollowLogicStatusOnDrawingTimeline_颜色跟随同轮逻辑题(t *testing.T) {
	logic := []*IntelCheckTimelinePoint{
		roundPoint(1, 10, IntelCheckStatusPass),
		roundPoint(3, 11, IntelCheckStatusPass),
		roundPoint(5, 12, IntelCheckStatusFail),
		roundPoint(7, 13, IntelCheckStatusRequestError),
	}
	drawing := []*IntelCheckTimelinePoint{
		roundPoint(2, 10, IntelCheckStatusFail),       // 结构未通过，逻辑题对 → 绿
		roundPoint(4, 11, IntelCheckStatusUnverified), // 未验证，逻辑题对 → 绿
		roundPoint(6, 12, IntelCheckStatusPass),       // 绘图通过，逻辑题错 → 红
		roundPoint(8, 13, IntelCheckStatusPass),       // 逻辑题请求失败 → 琥珀
	}

	out := followLogicStatusOnDrawingTimeline(logic, drawing)

	require.Equal(t, []string{
		IntelCheckStatusPass, IntelCheckStatusPass, IntelCheckStatusFail, IntelCheckStatusRequestError,
	}, statusesOf(out))
	// 点击色块看到的是哪一次检测不变。
	require.EqualValues(t, 2, out[0].ResultID)
	require.EqualValues(t, 8, out[3].ResultID)
}

func TestFollowLogicStatusOnDrawingTimeline_不改入参(t *testing.T) {
	logic := []*IntelCheckTimelinePoint{roundPoint(1, 10, IntelCheckStatusPass)}
	drawing := []*IntelCheckTimelinePoint{roundPoint(2, 10, IntelCheckStatusFail)}

	_ = followLogicStatusOnDrawingTimeline(logic, drawing)

	require.Equal(t, IntelCheckStatusFail, drawing[0].Status, "入参点不得被原地修改")
}

func TestFollowLogicStatusOnDrawingTimeline_无法跟随时保持原状态(t *testing.T) {
	logic := []*IntelCheckTimelinePoint{
		roundPoint(1, 10, IntelCheckStatusPass),
		roundPoint(3, 11, IntelCheckStatusRunning),
	}
	drawing := []*IntelCheckTimelinePoint{
		roundPoint(2, 10, IntelCheckStatusRunning), // 绘图还在检测：不提前染色
		roundPoint(4, 11, IntelCheckStatusFail),    // 同轮逻辑题还在检测：无结论可跟
		roundPoint(6, 99, IntelCheckStatusFail),    // 找不到同轮逻辑题
		roundPoint(8, 0, IntelCheckStatusFail),     // 没有轮次信息
	}

	out := followLogicStatusOnDrawingTimeline(logic, drawing)

	require.Equal(t, []string{
		IntelCheckStatusRunning, IntelCheckStatusFail, IntelCheckStatusFail, IntelCheckStatusFail,
	}, statusesOf(out))
}

func TestFollowLogicStatusOnDrawingTimeline_空输入(t *testing.T) {
	require.Empty(t, followLogicStatusOnDrawingTimeline(nil, nil))
	drawing := []*IntelCheckTimelinePoint{roundPoint(2, 10, IntelCheckStatusFail)}
	require.Equal(t, drawing, followLogicStatusOnDrawingTimeline(nil, drawing), "没有逻辑题时原样返回")
}

// 端到端：色块跟随逻辑题，但降智状态与统计口径都不被绘图结果污染。
func TestIntelCheckPublicOverview_绘图色块跟随逻辑题且状态统计只看逻辑题(t *testing.T) {
	svc, repo := intelCheckPublicFixture(t, true)
	repo.timelines[1] = map[string][]*IntelCheckTimelinePoint{
		IntelCheckKindLogic: {
			roundPoint(1, 10, IntelCheckStatusPass),
			roundPoint(3, 11, IntelCheckStatusPass),
		},
		IntelCheckKindDrawing: {
			roundPoint(2, 10, IntelCheckStatusFail),
			roundPoint(4, 11, IntelCheckStatusFail),
		},
	}

	out, err := svc.PublicOverview(context.Background())
	require.NoError(t, err)

	first := out.Groups[0]
	require.Equal(t, []string{IntelCheckStatusPass, IntelCheckStatusPass}, statusesOf(first.DrawingTimeline),
		"绘图在库里判失败，展示仍为绿色")
	require.Equal(t, IntelCheckStateNormal, first.State, "绘图连续失败也不触发降智")
	require.Equal(t, first.LogicStats24h, first.DrawingStats24h)
}
