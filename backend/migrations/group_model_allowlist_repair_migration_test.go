package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelAllowlistRepairMigration(t *testing.T) {
	content, err := FS.ReadFile("236_group_model_allowlist_repair.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	// 迁移必须保留旧模型广场列，并补建 model_allowlist。
	require.NotContains(t, sql, "RENAME COLUMN models_list_config TO model_allowlist")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "jsonb_build_object")
	require.Contains(t, sql, "ALTER TABLE groups ALTER COLUMN model_allowlist SET NOT NULL")
	require.Contains(t, sql, "COMMENT ON COLUMN groups.model_allowlist")
}
