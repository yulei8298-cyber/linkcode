import { spawn } from 'node:child_process'
import http from 'node:http'
import { fileURLToPath } from 'node:url'

const port = Number.parseInt(process.env.PORT || '8081', 10)
const maxBodyBytes = 300 * 1024
const maxOutputBytes = 64 * 1024
const maxConcurrent = 2
const maxQueued = 12
const hardTimeoutMs = 6500
const workingDirectory = fileURLToPath(new URL('.', import.meta.url))
let active = 0
const waiters = []

function send(response, status, payload) {
  const body = JSON.stringify(payload)
  response.writeHead(status, {
    'content-type': 'application/json; charset=utf-8',
    'content-length': Buffer.byteLength(body),
  })
  response.end(body)
}

async function readJSON(request) {
  const chunks = []
  let size = 0
  for await (const chunk of request) {
    size += chunk.length
    if (size > maxBodyBytes) throw new Error('请求体超过上限')
    chunks.push(chunk)
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'))
}

function acquireSlot() {
  if (active < maxConcurrent) {
    active += 1
    return Promise.resolve(true)
  }
  if (waiters.length >= maxQueued) return Promise.resolve(false)
  return new Promise((resolve) => waiters.push(resolve))
}

function releaseSlot() {
  const next = waiters.shift()
  if (next) {
    next(true)
    return
  }
  active -= 1
}

function killProcessGroup(child) {
  if (!child.pid) return
  try {
    process.kill(-child.pid, 'SIGKILL')
  } catch {
    child.kill('SIGKILL')
  }
}

function evaluateInWorker(html) {
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ['worker.mjs'], {
      cwd: workingDirectory,
      detached: true,
      stdio: ['pipe', 'pipe', 'ignore'],
    })
    const stdout = []
    let outputSize = 0
    let settled = false
    const finish = (error, result) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      if (error) reject(error)
      else resolve(result)
    }
    const timer = setTimeout(() => {
      killProcessGroup(child)
      finish(new Error('动作验收超时'))
    }, hardTimeoutMs)

    child.stdout.on('data', (chunk) => {
      outputSize += chunk.length
      if (outputSize > maxOutputBytes) {
        killProcessGroup(child)
        finish(new Error('动作验收输出超过上限'))
        return
      }
      stdout.push(chunk)
    })
    child.on('error', (error) => finish(error))
    child.on('exit', (code) => {
      if (code !== 0) return finish(new Error(`动作验收工作进程退出码 ${code}`))
      try {
        finish(null, JSON.parse(Buffer.concat(stdout).toString('utf8')))
      } catch {
        finish(new Error('动作验收工作进程返回无效 JSON'))
      }
    })
    child.stdin.end(JSON.stringify({ html }))
  })
}

const server = http.createServer(async (request, response) => {
  if (request.method === 'GET' && request.url === '/health') return send(response, 200, { ok: true })
  if (request.method !== 'POST' || request.url !== '/evaluate') return send(response, 404, { error: 'not found' })

  const acquired = await acquireSlot()
  if (!acquired) return send(response, 503, { error: 'busy' })
  try {
    const body = await readJSON(request)
    if (typeof body.html !== 'string' || body.html.length === 0 || Buffer.byteLength(body.html) > 256 * 1024) {
      return send(response, 400, { error: 'invalid html' })
    }
    send(response, 200, await evaluateInWorker(body.html))
  } catch (error) {
    send(response, 500, { error: error instanceof Error ? error.message.slice(0, 120) : 'evaluation failed' })
  } finally {
    releaseSlot()
  }
})

server.listen(port, '0.0.0.0')
process.on('SIGTERM', () => server.close())
process.on('SIGINT', () => server.close())
