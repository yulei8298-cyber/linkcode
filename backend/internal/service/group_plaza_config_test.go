//go:build unit

package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupPlazaConfigRoundTrip(t *testing.T) {
	for _, raw := range []string{
		`{"enabled":false,"plaza_enabled":false,"plaza_models":[]}`,
		`{"enabled":true,"models":["gateway-only"],"plaza_enabled":true,"plaza_models":[" gpt-5 ","gpt-5","*"]}`,
	} {
		var cfg GroupModelsListConfig
		require.NoError(t, json.Unmarshal([]byte(raw), &cfg))
		normalized := normalizeGroupModelsListConfig(cfg)
		require.Equal(t, cfg.PlazaEnabled, normalized.PlazaEnabled)
		require.NotNil(t, normalized.PlazaModels)
		encoded, err := json.Marshal(normalized)
		require.NoError(t, err)
		var restored GroupModelsListConfig
		require.NoError(t, json.Unmarshal(encoded, &restored))
		require.Equal(t, normalized, restored)
		if cfg.Enabled {
			require.Equal(t, []string{"gpt-5"}, *restored.PlazaModels)
			require.Equal(t, cfg.Models, restored.Models)
		} else {
			require.Empty(t, *restored.PlazaModels)
		}
	}
}

func TestListPlazaGroupsVisibilityAndSelection(t *testing.T) {
	hidden := false
	selected := []string{"gpt-selected", "gpt-manual"}
	empty := []string{}
	cases := []struct {
		name string
		cfg  GroupModelsListConfig
		want []string
	}{
		{"legacy automatic", GroupModelsListConfig{}, []string{"gpt-other", "gpt-selected"}},
		{"hidden despite channel", GroupModelsListConfig{PlazaEnabled: &hidden}, nil},
		{"custom filters channel and gateway models", GroupModelsListConfig{Models: []string{"gateway-only"}, PlazaModels: &selected}, []string{"gpt-manual", "gpt-selected"}},
		{"explicit empty hides", GroupModelsListConfig{PlazaModels: &empty}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newPlazaService([]Channel{plazaPricedChannel(1, "channel", []int64{10}, "openai", "gpt-selected", "gpt-other")}, []Group{{ID: 10, Platform: "openai", ModelsListConfig: tc.cfg}}, nil)
			groups, err := svc.ListGroups(context.Background())
			require.NoError(t, err)
			if len(tc.want) == 0 {
				require.Empty(t, groups)
				return
			}
			require.Len(t, groups, 1)
			names := []string{}
			for _, model := range groups[0].Models {
				names = append(names, model.Name)
			}
			require.Equal(t, tc.want, names)
		})
	}
}
