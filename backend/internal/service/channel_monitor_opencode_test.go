package service

import "testing"

func TestChannelMonitorOpenCodeGoIsQuotaOnly(t *testing.T) {
	if err := validateProvider(MonitorProviderOpenCodeGo); err != nil {
		t.Fatalf("OpenCode Go provider should be recognized: %v", err)
	}
	if err := validateCheckMode(MonitorProviderOpenCodeGo, MonitorCheckModeQuota); err != nil {
		t.Fatalf("OpenCode Go quota mode should be accepted: %v", err)
	}
	for _, mode := range []string{MonitorCheckModeProbe, MonitorCheckModeQuotaProbe} {
		if err := validateCheckMode(MonitorProviderOpenCodeGo, mode); err == nil {
			t.Fatalf("OpenCode Go %s mode must remain quota-only", mode)
		}
	}
}

func TestMonitorAccountQuotaCapabilityOpenCodeGoZenAndGo(t *testing.T) {
	for _, mode := range []string{AccountModeZen, AccountModeGo} {
		account := &Account{
			Platform:    PlatformOpenCodeGo,
			Credentials: map[string]any{"account_mode": mode},
		}
		if err := monitorAccountQuotaCapability(account); err != nil {
			t.Fatalf("OpenCode Go %s account should be quota-capable: %v", mode, err)
		}
	}
}
