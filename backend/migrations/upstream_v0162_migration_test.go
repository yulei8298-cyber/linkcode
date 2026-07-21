package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamV0162MigrationsFollowLocalCustomMigrations(t *testing.T) {
	entries, err := FS.ReadDir(".")
	require.NoError(t, err)

	indexes := make(map[string]int, len(entries))
	for i, entry := range entries {
		indexes[entry.Name()] = i
	}

	require.Contains(t, indexes, "183_prompt_audit.sql")
	require.Contains(t, indexes, "184_prompt_audit_full_prompt.sql")
	require.Contains(t, indexes, "185_redeem_code_affiliate_rebate_base.sql")
	require.Contains(t, indexes, "186_backfill_historical_redeem_code_rebate_bases.sql")
	require.Contains(t, indexes, "187_ops_ingress_reject_aggregates.sql")
	require.Contains(t, indexes, "188_auth_cache_invalidation_outbox.sql")
	require.Less(t, indexes["186_backfill_historical_redeem_code_rebate_bases.sql"], indexes["187_ops_ingress_reject_aggregates.sql"])
	require.Less(t, indexes["187_ops_ingress_reject_aggregates.sql"], indexes["188_auth_cache_invalidation_outbox.sql"])

	_, err = FS.ReadFile("183_ops_ingress_reject_aggregates.sql")
	require.Error(t, err)
	_, err = FS.ReadFile("184_auth_cache_invalidation_outbox.sql")
	require.Error(t, err)
}
