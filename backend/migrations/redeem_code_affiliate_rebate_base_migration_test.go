package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration186BackfillsOnlyConfirmedUnusedRedeemCodeTiers(t *testing.T) {
	content, err := FS.ReadFile("186_backfill_historical_redeem_code_rebate_bases.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "UPDATE redeem_codes")
	require.Contains(t, sql, "WHEN 20.00000000 THEN 20.00000000")
	require.Contains(t, sql, "WHEN 68.00000000 THEN 50.00000000")
	require.Contains(t, sql, "WHEN 138.00000000 THEN 100.00000000")
	require.Contains(t, sql, "WHEN 278.00000000 THEN 200.00000000")
	require.Contains(t, sql, "type = 'balance'")
	require.Contains(t, sql, "status = 'unused'")
	require.Contains(t, sql, "used_by IS NULL")
	require.Contains(t, sql, "used_at IS NULL")
	require.Contains(t, sql, "affiliate_rebate_base_amount IS NULL")
}
