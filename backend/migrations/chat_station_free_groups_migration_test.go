package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatStationFreeGroupsMigration(t *testing.T) {
	content, err := FS.ReadFile("177_chat_station_free_groups.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS is_hidden BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS is_free BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS daily_free_limit_usd DECIMAL(20, 10)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS chat_station_only BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "groups_daily_free_config_check")
	require.Contains(t, sql, "subscription_type = 'standard'")
	require.Contains(t, sql, "daily_free_limit_usd > 0")
	require.Contains(t, sql, "api_keys_chat_station_auto_unique_active")
	require.Contains(t, sql, "WHERE deleted_at IS NULL AND status = 'active' AND name = 'LobeHub Chat Station'")
	require.Contains(t, sql, "name = 'LobeHub Chat Station'")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS daily_free_usages")
	require.Contains(t, sql, "UNIQUE (user_id, group_id, usage_date)")
	require.Contains(t, sql, "CHECK (usage_usd >= 0)")
}

func TestChatStationFreeGroupsMigrationOnlyRetiresLegacyAutoSubscriptions(t *testing.T) {
	content, err := FS.ReadFile("177_chat_station_free_groups.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "g.name = 'OpenAI-chat'")
	require.Contains(t, sql, "g.platform = 'openai'")
	require.Contains(t, sql, "us.assigned_by IS NULL")
	require.Contains(t, sql, "us.notes = 'auto assigned by LobeHub SSO'")
	require.Contains(t, sql, "us.deleted_at IS NULL")
	require.Contains(t, sql, "AND NOT EXISTS ( SELECT 1 FROM user_subscriptions AS us WHERE us.group_id = g.id")
	require.NotContains(t, strings.ToUpper(sql), "DELETE FROM USER_SUBSCRIPTIONS")
}
