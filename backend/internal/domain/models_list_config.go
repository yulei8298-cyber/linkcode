package domain

// GroupModelsListConfig controls the optional custom /v1/models response list.
type GroupModelsListConfig struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models,omitempty"`
	// PlazaEnabled controls whether this group is included in the public model plaza.
	PlazaEnabled *bool `json:"plaza_enabled,omitempty"`
	// PlazaModels optionally replaces the gateway model list for plaza display.
	PlazaModels *[]string `json:"plaza_models,omitempty"`
}
