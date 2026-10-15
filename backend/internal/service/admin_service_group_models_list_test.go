//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 分组「模型列表 / 展示到模型广场」配置必须随创建、更新请求落库。
// 回归：0.2.4 同步后写入路径丢失，后台保存被静默忽略，广场一直用旧清单。

func TestAdminService_CreateGroup_PersistsModelsListConfig(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 61}
	svc := &adminServiceImpl{groupRepo: repo}
	plazaEnabled := true

	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name:           "grok-plaza-group",
		Platform:       PlatformGrok,
		RateMultiplier: 1,
		ModelsListConfig: GroupModelsListConfig{
			Enabled:      true,
			Models:       []string{" grok-4.7 ", "grok-4.6", "grok-4.7", "  "},
			PlazaEnabled: &plazaEnabled,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.True(t, repo.created.ModelsListConfig.Enabled)
	require.Equal(t, []string{"grok-4.7", "grok-4.6"}, repo.created.ModelsListConfig.Models, "去空白、去重并保持顺序")
	require.NotNil(t, repo.created.ModelsListConfig.PlazaEnabled)
	require.True(t, *repo.created.ModelsListConfig.PlazaEnabled)
}

func TestAdminService_UpdateGroup_PersistsModelsListConfig(t *testing.T) {
	existing := &Group{
		ID: 36, Name: "Grok", Platform: PlatformGrok, Status: StatusActive,
		ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-4.6", "grok-4.5"}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := &adminServiceImpl{groupRepo: repo}

	_, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{
		ModelsListConfig: &GroupModelsListConfig{Enabled: false, Models: []string{"grok-4.7", "grok-4.6", "grok-4.5"}},
	})

	require.NoError(t, err)
	require.NotNil(t, repo.updated)
	require.Equal(t, []string{"grok-4.7", "grok-4.6", "grok-4.5"}, repo.updated.ModelsListConfig.Models)
	require.False(t, repo.updated.ModelsListConfig.Enabled, "列表开关如实保存")
}

func TestAdminService_UpdateGroup_OmittedModelsListConfigIsUnchanged(t *testing.T) {
	existing := &Group{
		ID: 36, Name: "Grok", Platform: PlatformGrok, Status: StatusActive,
		ModelsListConfig: GroupModelsListConfig{Models: []string{"grok-4.6"}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := &adminServiceImpl{groupRepo: repo}
	description := "只改描述"

	_, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{Description: &description})

	require.NoError(t, err)
	require.NotNil(t, repo.updated)
	require.Equal(t, []string{"grok-4.6"}, repo.updated.ModelsListConfig.Models, "请求未携带该字段时不应被清空")
}

func TestCloneGroupModelsListConfig_IsDeepCopy(t *testing.T) {
	enabled := true
	plaza := []string{"a", "b"}
	source := GroupModelsListConfig{Enabled: true, Models: []string{"x", "y"}, PlazaEnabled: &enabled, PlazaModels: &plaza}

	cloned := cloneGroupModelsListConfig(source)
	cloned.Models[0] = "changed"
	(*cloned.PlazaModels)[0] = "changed"
	*cloned.PlazaEnabled = false

	require.Equal(t, []string{"x", "y"}, source.Models)
	require.Equal(t, []string{"a", "b"}, *source.PlazaModels)
	require.True(t, *source.PlazaEnabled)
	require.Nil(t, cloneGroupModelsListConfig(GroupModelsListConfig{}).Models, "空配置保持 nil，避免序列化出空数组")
}
