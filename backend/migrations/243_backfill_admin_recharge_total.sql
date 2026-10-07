-- 累计充值补齐管理员加款：此前管理员调余额不计入 users.total_recharged。
-- 管理员每次调整都会留一条 admin_balance 记录（value 为余额变化量），
-- 把其中的正向调整补进累计充值；扣减不回退累计值，与运行时口径一致。
UPDATE users AS u
SET total_recharged = u.total_recharged + a.amount
FROM (
    SELECT used_by, SUM(value) AS amount
    FROM redeem_codes
    WHERE type = 'admin_balance' AND status = 'used' AND value > 0 AND used_by IS NOT NULL
    GROUP BY used_by
) AS a
WHERE u.id = a.used_by;
