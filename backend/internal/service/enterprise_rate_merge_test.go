package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeEnterpriseGroupRates(t *testing.T) {
	groupRates := map[int64]float64{38: 0.3, 2: 0.17, 45: 0.15}

	t.Run("企业倍率低于分组默认倍率时采用并标记", func(t *testing.T) {
		merged, marked := MergeEnterpriseGroupRates(nil, map[int64]float64{38: 0.28}, groupRates)
		require.Equal(t, map[int64]float64{38: 0.28}, merged)
		require.Equal(t, map[int64]bool{38: true}, marked)
	})

	t.Run("个人专属倍率更低时保持个人倍率，不标记", func(t *testing.T) {
		merged, marked := MergeEnterpriseGroupRates(map[int64]float64{38: 0.2}, map[int64]float64{38: 0.28}, groupRates)
		require.Equal(t, map[int64]float64{38: 0.2}, merged)
		require.Empty(t, marked)
	})

	t.Run("个人专属倍率更高时，以企业倍率为准", func(t *testing.T) {
		merged, marked := MergeEnterpriseGroupRates(map[int64]float64{38: 0.5}, map[int64]float64{38: 0.28}, groupRates)
		require.Equal(t, 0.28, merged[38])
		require.True(t, marked[38])
	})

	t.Run("企业倍率不低于分组默认倍率时不改变", func(t *testing.T) {
		merged, marked := MergeEnterpriseGroupRates(nil, map[int64]float64{2: 0.17, 45: 0.2}, groupRates)
		require.Empty(t, merged)
		require.Empty(t, marked)
	})

	t.Run("不在用户可见分组里的企业倍率没有对照基准，忽略", func(t *testing.T) {
		merged, marked := MergeEnterpriseGroupRates(nil, map[int64]float64{999: 0.1}, groupRates)
		require.Empty(t, merged)
		require.Empty(t, marked)
	})

	t.Run("保留其他分组的个人倍率且不修改入参", func(t *testing.T) {
		personal := map[int64]float64{2: 0.1}
		merged, _ := MergeEnterpriseGroupRates(personal, map[int64]float64{38: 0.28}, groupRates)
		require.Equal(t, map[int64]float64{2: 0.1, 38: 0.28}, merged)
		require.Equal(t, map[int64]float64{2: 0.1}, personal, "入参不应被修改")
	})

	t.Run("没有企业倍率时原样返回", func(t *testing.T) {
		personal := map[int64]float64{2: 0.1}
		merged, marked := MergeEnterpriseGroupRates(personal, nil, groupRates)
		require.Equal(t, personal, merged)
		require.Nil(t, marked)
	})
}
