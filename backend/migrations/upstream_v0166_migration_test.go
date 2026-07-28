package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamV0166MigrationsFollowPublishedLocalHistory(t *testing.T) {
	entries, err := FS.ReadDir(".")
	require.NoError(t, err)

	indexes := make(map[string]int, len(entries))
	for i, entry := range entries {
		indexes[entry.Name()] = i
	}

	ordered := []string{
		"188_auth_cache_invalidation_outbox.sql",
		"189_composite_model_routes.sql",
		"190_group_reasoning_effort_policy.sql",
		"191_alipay_mobile_precreate_deep_link.sql",
		"192_add_usage_log_session_id.sql",
		"193_allow_live_usage_request_type.sql",
		"194_add_group_allow_live.sql",
		"195_add_users_email_alias_dedup_index_notx.sql",
		"196_extend_group_auth_cache_invalidation.sql",
	}
	for i, name := range ordered {
		require.Contains(t, indexes, name)
		if i > 0 {
			require.Less(t, indexes[ordered[i-1]], indexes[name])
		}
	}

	for _, collided := range []string{
		"172_composite_model_routes.sql",
		"185_group_reasoning_effort_policy.sql",
		"186_alipay_mobile_precreate_deep_link.sql",
		"186_group_auth_cache_image_generation.sql",
		"187_add_usage_log_session_id.sql",
		"188_allow_live_usage_request_type.sql",
		"189_add_group_allow_live.sql",
		"190_add_users_email_alias_dedup_index_notx.sql",
	} {
		_, err = FS.ReadFile(collided)
		require.Error(t, err, "colliding upstream migration must be renumbered: %s", collided)
	}
}

func TestUpstreamV0166AuthCacheMigrationPreservesLocalAndUpstreamFields(t *testing.T) {
	content, err := FS.ReadFile("196_extend_group_auth_cache_invalidation.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	for _, field := range []string{
		"is_hidden",
		"is_free",
		"daily_free_limit_usd",
		"chat_station_only",
		"ip_whitelist",
		"ip_blacklist",
		"allow_image_generation",
		"allow_live",
		"max_reasoning_effort",
		"reasoning_effort_mappings",
	} {
		require.Contains(t, sql, "OLD."+field+" IS NOT DISTINCT FROM NEW."+field)
	}
}
