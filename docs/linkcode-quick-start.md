# LinkCode 快速开始

LinkCode 是一个聚合多家上游渠道的 AI API 网关，支持 **OpenAI / ChatGPT 兼容协议** 与 **Anthropic / Claude 兼容协议**。你只需要完成充值、创建 API 密钥、复制接口地址三步，就可以在 Codex CLI、Claude Code、OpenCode 或自己的程序里使用。

> 本教程中的 `sk-你的密钥` 请替换为你在控制台创建的真实 API Key。不要把密钥发给他人，也不要提交到公开仓库。

![LinkCode 首页](/tutorial/01-home-safe.png)

---

## 一、登录控制台

1. 打开 [LinkCode 控制台](https://api.linkcode.site/)。
2. 点击右上角「登录」。
3. 输入邮箱和密码后进入仪表盘。

![登录 LinkCode](/tutorial/02-login-safe.png)

登录后会看到余额、今日请求、今日消费、Token 使用趋势等账户概览。后续查看用量、创建密钥、兑换充值都从左侧菜单进入。

![控制台仪表盘](/tutorial/03-dashboard-safe.png)

---

## 二、充值或兑换额度

如果你已有兑换码，进入「兑换」页面输入兑换码并确认。兑换成功后，额度会自动加入账户余额。

![兑换页面](/tutorial/06-redeem-safe.png)

如果需要购买额度，可以在公开页面查看「定价方案」，确认充值比例、赠送额度和渠道倍率后点击「前往充值」。

![定价方案](/tutorial/07-pricing-safe.png)

---

## 三、创建 API 密钥

1. 在左侧菜单进入「API 密钥」。
2. 点击右上角「创建密钥」。
3. 填写密钥名称。
4. 选择分组。不同分组对应不同渠道、倍率和可用模型。
5. 按需设置 IP 限制、额度限制、速率限制和有效期。
6. 点击「创建」，创建成功后立即复制并保存密钥。

![API 密钥列表](/tutorial/04-api-keys-safe.png)

![创建 API 密钥](/tutorial/05-create-key-safe.png)

> API Key 通常只在创建时完整显示。后续页面会以 `sk-xxx...xxxx` 的形式脱敏展示，请创建后立刻保存。

---

## 四、复制接口地址

在「API 密钥」页面顶部可以直接复制常用端点：

| 用途 | Base URL |
| --- | --- |
| OpenAI / ChatGPT 兼容接口 | `https://api.linkcode.site/v1` |
| Claude / Anthropic 兼容接口 | `https://api.linkcode.site` |
| OpenAI 国内直连快速端点 | `https://api-fast.linkcode.site/v1` |
| Claude 国内直连快速端点 | `https://api-fast.linkcode.site` |

鉴权方式与官方协议保持一致：

- OpenAI 协议：请求头使用 `Authorization: Bearer sk-你的密钥`
- Claude 协议：请求头使用 `x-api-key: sk-你的密钥`

---

## 五、接入 Codex CLI / OpenCode

在「API 密钥」页面点击某个密钥右侧的「使用密钥」，控制台会自动生成 Codex CLI、Codex CLI WebSocket 和 OpenCode 的配置示例。按你的系统选择 macOS / Linux 或 Windows，然后复制配置到对应文件。

![使用 API 密钥](/tutorial/08-use-key-safe.png)

Codex CLI 常用配置目录：

```bash
mkdir -p ~/.codex
```

`~/.codex/config.toml` 示例：

```toml
model_provider = "OpenAI"
model = "gpt-5.5"
review_model = "gpt-5.5"
model_reasoning_effort = "xhigh"
disable_response_storage = true
network_access = "enabled"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://api.linkcode.site"
wire_api = "responses"
requires_openai_auth = true
```

`~/.codex/auth.json` 示例：

```json
{
  "OPENAI_API_KEY": "sk-你的密钥"
}
```

---

## 六、接入 Claude Code

如果你的密钥分组是 Claude / Anthropic 协议，使用 Claude Code 时设置下面两个环境变量即可：

```bash
export ANTHROPIC_BASE_URL="https://api.linkcode.site"
export ANTHROPIC_API_KEY="sk-你的密钥"

claude
```

如需长期生效，可以把上面的 `export` 写入 `~/.zshrc` 或 `~/.bashrc`。

---

## 七、接入 OpenAI 兼容客户端

任何支持自定义 Base URL 的 OpenAI 兼容客户端都可以使用 LinkCode。以 Python SDK 为例：

```python
from openai import OpenAI

client = OpenAI(
    base_url="https://api.linkcode.site/v1",
    api_key="sk-你的密钥",
)

resp = client.chat.completions.create(
    model="gpt-5.5",
    messages=[{"role": "user", "content": "你好，介绍一下 LinkCode"}],
)

print(resp.choices[0].message.content)
```

---

## 八、用 curl 验证

OpenAI / ChatGPT 兼容接口：

```bash
curl https://api.linkcode.site/v1/chat/completions \
  -H "Authorization: Bearer sk-你的密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.5",
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

Claude / Anthropic 兼容接口：

```bash
curl https://api.linkcode.site/v1/messages \
  -H "x-api-key: sk-你的密钥" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "你好"}]
  }'
```

返回正常 JSON 响应就表示接入成功。

---

## 常见问题

**Q：返回 401 或鉴权失败？**  
检查密钥是否复制完整、是否用错请求头。OpenAI 用 `Authorization: Bearer`，Claude 用 `x-api-key`。

**Q：提示没有可用分组或模型不可用？**  
进入「API 密钥」页面确认密钥分组是否正确，也可以在「定价方案」或「渠道状态」查看当前可用渠道。

**Q：应该使用哪个 Base URL？**  
OpenAI 兼容客户端使用 `https://api.linkcode.site/v1`。Claude / Anthropic 客户端使用 `https://api.linkcode.site`。如果你的网络更适合国内直连，可以尝试 `api-fast.linkcode.site` 对应端点。

**Q：如何查看消费明细？**  
进入控制台左侧「使用记录」，可以按时间、模型、API Key 查看请求、Token 与扣费明细。
