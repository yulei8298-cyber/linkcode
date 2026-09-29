import assert from 'node:assert/strict'
import fs from 'node:fs'
import vm from 'node:vm'
import { test } from 'node:test'

// 仓库只保存画布构建产物；执行实际发布的创建/轮询函数，验证协议与鉴权。
const publicRoot = new URL('../../frontend/public/infinite-canvas/', import.meta.url)
const html = fs.readFileSync(new URL('index.html', publicRoot), 'utf8')
const asset = html.match(/src="\/infinite-canvas-static\/(assets\/index-[^"?]+\.js)(?:\?[^" ]*)?"/)[1]
const bundle = fs.readFileSync(new URL(asset, publicRoot), 'utf8')
const start = bundle.indexOf('async function D_t(')
const end = bundle.indexOf('async function U_t(', start)
assert.ok(start >= 0 && end > start, '画布视频函数边界已变更，需同步更新兼容测试')

const config = { baseUrl: 'https://relay.example/v1', apiKey: 'test-key', videoSeconds: 6, size: '1280x720', vquality: '720p' }
const task = { id: 'video-task', model: 'grok-imagine-video', provider: 'openai' }

test('入口与懒加载块引用同一个带版本的主模块，避免重复初始化', () => {
  const entryPath = html.match(/src="([^" ]+\/assets\/index-[^" ]+\.js(?:\?[^" ]*)?)"/)[1]
  const entryURL = new URL(entryPath, 'https://relay.example')
  const childPath = bundle.match(/import\("(\.\/highlighted-body-[^"]+)"\)/)[1]
  const childURL = new URL(childPath, entryURL)
  const child = fs.readFileSync(new URL(`assets/${childURL.pathname.split('/').pop()}`, publicRoot), 'utf8')
  const parentPath = child.match(/from"(\.\/index-[^"]+)"/)[1]
  assert.equal(new URL(parentPath, childURL).href, entryURL.href)
  assert.ok(entryURL.search, '主模块更新必须避开旧缓存')
  assert.equal(childURL.search, entryURL.search)
})

function client({ created = { request_id: task.id }, status = { status: 'pending' } } = {}) {
  const requests = []
  const video = new Blob(['video-content'], { type: 'video/mp4' })
  const signal = new AbortController().signal
  const context = vm.createContext({
    FormData, Blob, DOMException,
    li: (key) => key,
    Zg: (model) => model,
    dB: (settings, path) => settings.baseUrl + path,
    fB: (settings) => ({ Authorization: `Bearer ${settings.apiKey}` }),
    Ir: {
      isCancel: () => false,
      isAxiosError: () => false,
      async post(url, body, options) { requests.push({ method: 'POST', url, body, options }); return { data: created } },
      async get(url, options) { requests.push({ method: 'GET', url, options }); return { data: url.endsWith('/content') ? video : status } },
    },
  })
  vm.runInContext(bundle.slice(start, end), context)
  return { context, requests, video, signal }
}

test('xAI request_id 可用作画布任务 ID，提交保留参数与鉴权', async () => {
  const c = client()
  const result = await c.context.D_t(config, task.model, 'waves', [], { signal: c.signal })
  assert.equal(result.id, task.id)
  assert.equal(c.requests[0].body.get('seconds'), '6')
  assert.equal(c.requests[0].body.get('resolution_name'), '720p')
  assert.equal(c.requests[0].options.headers.Authorization, 'Bearer test-key')
  assert.equal(c.requests[0].options.signal, c.signal)
})

test('继续兼容已有 OpenAI id', async () => {
  const c = client({ created: { id: 'openai-task' } })
  assert.equal((await c.context.D_t(config, 'sora', 'waves', [])).id, 'openai-task')
})

test('xAI done 经带鉴权的内容接口下载，忽略上游直链', async () => {
  const c = client({ status: { status: 'done', video: { url: '/v1/videos/video-task/content' }, url: 'https://external.example/movie.mp4' } })
  const result = await c.context.L_t(config, task, { signal: c.signal })
  assert.equal(result.status, 'completed')
  assert.equal(result.result.blob, c.video)
  assert.equal(c.requests.length, 2)
  assert.equal(c.requests[1].url, 'https://relay.example/v1/videos/video-task/content')
  assert.equal(c.requests[1].options.headers.Authorization, 'Bearer test-key')
  assert.equal(c.requests[1].options.responseType, 'blob')
  assert.equal(c.requests[1].options.signal, c.signal)
})

test('OpenAI completed 仍走原有鉴权下载', async () => {
  const c = client({ status: { status: 'completed' } })
  assert.equal((await c.context.L_t(config, task)).result.blob, c.video)
  assert.equal(c.requests[1].options.headers.Authorization, 'Bearer test-key')
})

test('映射到 Grok 的自定义模型别名也能识别本网关内容地址', async () => {
  for (const prefix of ['/v1', '']) {
    const c = client({ status: { status: 'done', video: { url: `${prefix}/videos/video-task/content` } } })
    const result = await c.context.L_t(config, { ...task, model: 'custom-video-alias' })
    assert.equal(result.status, 'completed')
    assert.equal(result.result.blob, c.video)
    assert.equal(c.requests[1].url, 'https://relay.example/v1/videos/video-task/content')
    assert.equal(c.requests[1].options.headers.Authorization, 'Bearer test-key')
  }
})

test('其他提供方的 done 顶层直链继续可用且不附带中转凭据', async () => {
  const url = 'https://external.example/movie.mp4'
  const c = client({ status: { status: 'done', url, video: { url } } })
  const result = await c.context.L_t(config, { ...task, model: 'other-video-model' })
  assert.equal(result.status, 'completed')
  assert.equal(result.result.url, url)
  assert.equal(c.requests[1].url, url)
  assert.equal(c.requests[1].options.headers, undefined)
})

test('pending 或尚无视频的 done 不提前下载', async () => {
  for (const status of [{ status: 'pending' }, { status: 'done', video: {} }]) {
    const c = client({ status })
    assert.equal((await c.context.L_t(config, task)).status, 'pending')
    assert.equal(c.requests.length, 1)
  }
})

test('xAI expired 终止轮询并显示错误', async () => {
  const c = client({ status: { status: 'expired', error: { message: 'Task expired' } } })
  const result = await c.context.L_t(config, task)
  assert.equal(result.status, 'failed')
  assert.equal(result.error, 'Task expired')
  assert.equal(c.requests.length, 1)
})

test('失败状态保留上游错误', async () => {
  const c = client({ status: { status: 'failed', error: { message: 'Generation rejected' } } })
  assert.equal((await c.context.L_t(config, task)).error, 'Generation rejected')
})

test('缺少任务标识时保留创建失败提示', async () => {
  const c = client({ created: {} })
  await assert.rejects(c.context.D_t(config, task.model, 'waves', []), /noVideoTaskId/)
})
