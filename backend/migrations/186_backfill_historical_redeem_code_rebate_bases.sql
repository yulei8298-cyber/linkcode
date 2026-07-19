-- Backfill the confirmed paid amounts for legacy promotional balance codes.
-- Only unused legacy rows without an explicitly configured rebate basis are
-- updated, so redeemed codes and later administrator changes are preserved.
UPDATE redeem_codes
SET affiliate_rebate_base_amount = CASE value
    WHEN 20.00000000 THEN 20.00000000
    WHEN 68.00000000 THEN 50.00000000
    WHEN 138.00000000 THEN 100.00000000
    WHEN 278.00000000 THEN 200.00000000
END
WHERE type = 'balance'
  AND status = 'unused'
  AND used_by IS NULL
  AND used_at IS NULL
  AND affiliate_rebate_base_amount IS NULL
  AND value IN (20.00000000, 68.00000000, 138.00000000, 278.00000000);
