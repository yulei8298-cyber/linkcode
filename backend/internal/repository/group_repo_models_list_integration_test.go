//go:build integration

package repository

import "github.com/Wei-Shaw/sub2api/internal/service"

// 回归：分组「模型列表 / 展示到模型广场」配置必须随创建和更新写入数据库。
func (s *GroupRepoSuite) TestCreateAndUpdate_PersistModelsListConfig() {
	plazaEnabled := true
	group := &service.Group{
		Name:             "models-list-persist",
		Platform:         service.PlatformGrok,
		RateMultiplier:   1,
		Status:           service.StatusActive,
		SubscriptionType: service.SubscriptionTypeStandard,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled:      false,
			Models:       []string{"grok-4.6"},
			PlazaEnabled: &plazaEnabled,
		},
	}
	s.Require().NoError(s.repo.Create(s.ctx, group), "Create")

	got, err := s.repo.GetByID(s.ctx, group.ID)
	s.Require().NoError(err)
	s.Require().Equal([]string{"grok-4.6"}, got.ModelsListConfig.Models)
	s.Require().NotNil(got.ModelsListConfig.PlazaEnabled)

	got.ModelsListConfig.Models = []string{"grok-4.7", "grok-4.6"}
	got.ModelsListConfig.Enabled = true
	s.Require().NoError(s.repo.Update(s.ctx, got), "Update")

	reloaded, err := s.repo.GetByID(s.ctx, group.ID)
	s.Require().NoError(err)
	s.Require().True(reloaded.ModelsListConfig.Enabled)
	s.Require().Equal([]string{"grok-4.7", "grok-4.6"}, reloaded.ModelsListConfig.Models)
}
