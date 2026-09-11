-- 236: 可重放地确保 model_allowlist 存在，并从旧 models_list_config 回填白名单字段。
DO $$
BEGIN
  ALTER TABLE groups ADD COLUMN IF NOT EXISTS model_allowlist JSONB NOT NULL DEFAULT '{}'::jsonb;
  IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='groups' AND column_name='models_list_config') THEN
    EXECUTE $q$
      UPDATE groups SET model_allowlist = jsonb_build_object(
        'enabled', COALESCE((models_list_config->>'enabled')::boolean, false),
        'models', COALESCE(models_list_config->'models', '[]'::jsonb)
      ) WHERE COALESCE(model_allowlist, '{}'::jsonb) = '{}'
    $q$;
  END IF;
END $$;
ALTER TABLE groups ALTER COLUMN model_allowlist SET DEFAULT '{}'::jsonb;
ALTER TABLE groups ALTER COLUMN model_allowlist SET NOT NULL;
COMMENT ON COLUMN groups.model_allowlist IS 'Group model allowlist: constrains both model listing responses and request admission';
