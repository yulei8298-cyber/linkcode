-- Store the actual paid amount used as the basis for inviter rebates. NULL is
-- intentionally retained for legacy codes, whose redemption path falls back
-- to the credited face value.
ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS affiliate_rebate_base_amount DECIMAL(20,8);

COMMENT ON COLUMN redeem_codes.affiliate_rebate_base_amount
    IS '邀请返利计算基数；NULL 表示历史兑换码按 value 计算';
