// 使用教程的内置默认内容（Markdown）。
// 后台「系统设置 → 使用教程内容」配置后会覆盖此默认值；未配置时公开门户展示这份。
// 这是针对 linkcode 自有网关编写的原创接入指南。

export const DEFAULT_TUTORIAL_MD = `# 快速接入指南

本站是一个聚合多家上游渠道的 AI API 网关，兼容 **Anthropic（Claude）** 与 **OpenAI（ChatGPT）** 两套官方 API 协议。你只需要把客户端的「接口地址」和「API Key」换成本站的，即可像调用官方 API 一样使用。

> 下文中的 \`https://你的站点域名\` 请替换为本站实际地址（例如浏览器地址栏里的域名）。

---

## 一、获取 API Key

1. 登录控制台，进入「**API 密钥**」页面。
2. 点击「**创建密钥**」，选择一个分组（不同分组对应不同的渠道与计费倍率）。
3. 复制生成的密钥，形如 \`sk-xxxxxxxxxxxxxxxx\`，妥善保管（仅在创建时完整显示）。

---

## 二、接口地址

| 协议 | Base URL | 说明 |
| --- | --- | --- |
| Anthropic（Claude） | \`https://你的站点域名\` | 调用 \`/v1/messages\` |
| OpenAI（ChatGPT） | \`https://你的站点域名/v1\` | 调用 \`/v1/chat/completions\` |

鉴权方式与官方一致：
- Anthropic 协议：请求头携带 \`x-api-key: 你的密钥\`
- OpenAI 协议：请求头携带 \`Authorization: Bearer 你的密钥\`

---

## 三、接入 Claude Code

Claude Code 通过环境变量指定网关地址和密钥即可：

\`\`\`bash
export ANTHROPIC_BASE_URL="https://你的站点域名"
export ANTHROPIC_API_KEY="你的密钥"

# 启动
claude
\`\`\`

如需常驻，可把以上 \`export\` 写入 \`~/.bashrc\` 或 \`~/.zshrc\`。

---

## 四、接入 ChatGPT / OpenAI 兼容客户端

任何支持「自定义 Base URL」的 OpenAI 客户端都可直接接入。以官方 \`openai\` Python SDK 为例：

\`\`\`python
from openai import OpenAI

client = OpenAI(
    base_url="https://你的站点域名/v1",
    api_key="你的密钥",
)

resp = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "你好"}],
)
print(resp.choices[0].message.content)
\`\`\`

---

## 五、用 curl 快速验证

**Claude（Anthropic 协议）：**

\`\`\`bash
curl https://你的站点域名/v1/messages \\
  -H "x-api-key: 你的密钥" \\
  -H "anthropic-version: 2023-06-01" \\
  -H "content-type: application/json" \\
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "你好"}]
  }'
\`\`\`

**ChatGPT（OpenAI 协议）：**

\`\`\`bash
curl https://你的站点域名/v1/chat/completions \\
  -H "Authorization: Bearer 你的密钥" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "你好"}]
  }'
\`\`\`

返回正常的 JSON 响应即表示接入成功。

---

## 六、常见问题

**Q：返回 401 / 鉴权失败？**
检查密钥是否复制完整、是否过期，以及是否用对了请求头（Claude 用 \`x-api-key\`，OpenAI 用 \`Authorization: Bearer\`）。

**Q：提示分组未分配 / 无可用渠道？**
该密钥所属分组当前没有可用渠道，或你的账号没有该分组权限，请在控制台确认分组与余额。

**Q：可以用哪些模型？**
可调用的模型取决于你所选分组绑定的渠道，具体见「定价方案」页或控制台内的模型列表。

**Q：余额与计费？**
按 Token 实时计费，不同分组有不同倍率，余额永久有效。用量明细可在控制台「使用记录」查看。
`
