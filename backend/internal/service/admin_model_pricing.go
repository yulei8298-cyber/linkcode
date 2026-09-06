package service

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// AdminModelPrice stores only edited fields. Nil inherits the current catalog;
// zero is a deliberate free rate. All token prices use USD per token.
type AdminModelPrice struct {
	Input        *float64 `json:"input_price,omitempty"`
	Output       *float64 `json:"output_price,omitempty"`
	CacheWrite   *float64 `json:"cache_write_price,omitempty"`
	CacheWrite1h *float64 `json:"cache_write_1h_price,omitempty"`
	CacheRead    *float64 `json:"cache_read_price,omitempty"`
	ImageInput   *float64 `json:"image_input_price,omitempty"`
	ImageOutput  *float64 `json:"image_output_price,omitempty"`
	PerImage     *float64 `json:"per_image_price,omitempty"`
}

func (p AdminModelPrice) validate() error {
	count := 0
	for _, v := range []*float64{p.Input, p.Output, p.CacheWrite, p.CacheWrite1h, p.CacheRead, p.ImageInput, p.ImageOutput, p.PerImage} {
		if v != nil {
			count++
			if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 {
				return fmt.Errorf("prices must be finite and non-negative")
			}
		}
	}
	if count == 0 {
		return fmt.Errorf("enter at least one price")
	}
	return nil
}

func adminPriceModelKey(model string) (string, error) {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" || len(model) > 256 || strings.ContainsAny(model, " \t\n\r*?") {
		return "", fmt.Errorf("invalid model name")
	}
	return model, nil
}

func (s *PricingService) adminPricingPath() string {
	return filepath.Join(s.cfg.Pricing.DataDir, "admin-pricing-overrides.json")
}

func (s *PricingService) loadAdminPricing() error {
	body, err := os.ReadFile(s.adminPricingPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read admin prices: %w", err)
	}
	var prices map[string]AdminModelPrice
	if err = json.Unmarshal(body, &prices); err != nil {
		return fmt.Errorf("decode admin prices: %w", err)
	}
	for model, p := range prices {
		key, err := adminPriceModelKey(model)
		if err != nil || key != model {
			return fmt.Errorf("invalid stored price model")
		}
		if err := p.validate(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.adminOverrides = prices
	s.mu.Unlock()
	return nil
}

// SetAdminModelPrice atomically persists before publishing to readers. Catalog
// refreshes update a separate map, so they cannot overwrite an admin edit.
// A nil price restores inheritance from the official catalog.
func (s *PricingService) SetAdminModelPrice(model string, price *AdminModelPrice) error {
	key, err := adminPriceModelKey(model)
	if err != nil {
		return err
	}
	if price != nil {
		if err := price.validate(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]AdminModelPrice, len(s.adminOverrides)+1)
	for k, v := range s.adminOverrides {
		next[k] = v
	}
	if price == nil {
		delete(next, key)
	} else {
		next[key] = *price
	}
	body, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(s.cfg.Pricing.DataDir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.cfg.Pricing.DataDir, ".admin-prices-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), s.adminPricingPath()); err != nil {
		return err
	}
	s.adminOverrides = next
	return nil
}

func (s *PricingService) adminPriceLocked(model string) (AdminModelPrice, bool) {
	for _, candidate := range s.buildModelLookupCandidates(strings.ToLower(strings.TrimSpace(model))) {
		if price, ok := s.adminOverrides[candidate]; ok {
			return price, true
		}
	}
	return AdminModelPrice{}, false
}

func (s *PricingService) applyAdminPricingLocked(model string, base *LiteLLMModelPricing) *LiteLLMModelPricing {
	p, ok := s.adminPriceLocked(model)
	if !ok {
		return base
	}
	var out LiteLLMModelPricing
	if base != nil {
		out = *base
	} else {
		out.TokenPricingAbsent = true
	}
	for _, pair := range []struct {
		src *float64
		dst *float64
	}{
		{p.Input, &out.InputCostPerToken}, {p.Output, &out.OutputCostPerToken},
		{p.CacheWrite, &out.CacheCreationInputTokenCost}, {p.CacheWrite1h, &out.CacheCreationInputTokenCostAbove1hr},
		{p.CacheRead, &out.CacheReadInputTokenCost}, {p.ImageInput, &out.InputCostPerImageToken},
		{p.ImageOutput, &out.OutputCostPerImageToken}, {p.PerImage, &out.OutputCostPerImage},
	} {
		if pair.src != nil {
			*pair.dst = *pair.src
		}
	}
	if p.Input != nil && p.Output != nil {
		out.TokenPricingAbsent = false
	}
	return &out
}

func (s *PricingService) GetAdminModelPrice(model string) (AdminModelPrice, bool, *LiteLLMModelPricing) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, edited := s.adminPriceLocked(model)
	base := s.getCatalogModelPricingLocked(model)
	if base != nil {
		copy := *base
		base = &copy
	}
	return p, edited, base
}
