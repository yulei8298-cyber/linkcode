# Responses input_tokens 健康度修复记录

## 问题

部分 Claude Code 请求会调用 `/v1/responses/input_tokens` 获取输入 token 估算。
OpenAI OAuth 上游通常不提供这个可选端点，返回：

```json
{"detail":"Not Found"}
```

旧逻辑把上游 404 包装为中转站 502，并按普通上游错误写入运维监控，导致渠道健康度被拉低。该请求不是主对话请求。

## 修改内容

1. 在 Responses 路由统一识别 `/responses/input_tokens` 及其路径前缀和尾部斜杠变体。
2. 中转站使用现有 tiktoken 逻辑本地估算并返回 `{"input_tokens": N}`，不选择账号、不占并发、不请求上游、不产生用量扣费。
3. 将该路径纳入 `is_count_tokens` 运维标记和默认错误过滤，后续即使参数异常也不会污染健康度统计。
4. 新增迁移 `221_mark_legacy_responses_input_tokens_errors.sql`，发布时自动把历史同路径错误补标为 `is_count_tokens=true`，不修改其他 404 记录。

## 发布说明

执行现有 Docker 更新流程即可触发迁移。迁移只修改运维错误日志的分类字段，不影响用户余额、API 密钥、账号状态和主 API 调用。

## 验证

已通过：

- OpenAI token 估算函数回归测试；
- Responses input_tokens 路径识别测试；
- 本地 handler 返回结构测试；
- 运维错误标记识别测试；
- `internal/server/routes` 测试。

完整包测试中存在仓库原有的 `httptest` 本地监听受当前沙箱限制问题，未能在当前环境完整跑通，与本次改动无关；相关包的全部测试二进制已完成编译检查。
