package service

import "strings"

func normalizeGroupModelsListConfig(cfg GroupModelsListConfig) GroupModelsListConfig {
	out := GroupModelsListConfig{Enabled: cfg.Enabled, PlazaEnabled: cfg.PlazaEnabled}
	if cfg.PlazaModels != nil {
		models := make([]string, 0, len(*cfg.PlazaModels))
		seen := make(map[string]bool)
		for _, name := range *cfg.PlazaModels {
			name = strings.TrimSpace(name)
			if name != "" && !seen[name] && !strings.ContainsAny(name, "*?") {
				models = append(models, name)
				seen[name] = true
			}
		}
		out.PlazaModels = &models
	}
	if len(cfg.Models) == 0 {
		return out
	}

	seen := make(map[string]struct{}, len(cfg.Models))
	out.Models = make([]string, 0, len(cfg.Models))
	for _, model := range cfg.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out.Models = append(out.Models, model)
	}
	if len(out.Models) == 0 {
		out.Models = nil
	}
	return out
}

func (g *Group) CustomModelsListEnabled() bool {
	return g != nil && g.ModelsListConfig.Enabled && len(g.ModelsListConfig.Models) > 0
}
