-- 235: 新增 groups.model_allowlist，保留 LinkCode 的 models_list_config（模型广场）列。
-- 白名单只复制 enabled/models，避免破坏 plaza_enabled/plaza_models 等二开字段。
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
COMMENT ON COLUMN groups.model_allowlist IS 'Group model allowlist: constrains both model listing responses and request admission';
