-- Keep the local durable auth-cache trigger aligned with every group field
-- stored in the API-key auth snapshot. This migration intentionally runs after
-- the reasoning policy and Live gate columns have been created.

CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.is_hidden IS NOT DISTINCT FROM NEW.is_hidden
       AND OLD.is_free IS NOT DISTINCT FROM NEW.is_free
       AND OLD.daily_free_limit_usd IS NOT DISTINCT FROM NEW.daily_free_limit_usd
       AND OLD.chat_station_only IS NOT DISTINCT FROM NEW.chat_station_only
       AND OLD.ip_whitelist IS NOT DISTINCT FROM NEW.ip_whitelist
       AND OLD.ip_blacklist IS NOT DISTINCT FROM NEW.ip_blacklist
       AND OLD.allow_image_generation IS NOT DISTINCT FROM NEW.allow_image_generation
       AND OLD.allow_live IS NOT DISTINCT FROM NEW.allow_live
       AND OLD.max_reasoning_effort IS NOT DISTINCT FROM NEW.max_reasoning_effort
       AND OLD.reasoning_effort_mappings IS NOT DISTINCT FROM NEW.reasoning_effort_mappings
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.group_id = target_group_id
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
