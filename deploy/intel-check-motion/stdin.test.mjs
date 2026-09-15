import assert from 'node:assert/strict'
import { Readable } from 'node:stream'
import test from 'node:test'
import { readJSONStdin } from './stdin.mjs'

test('从标准输入流读取 JSON 请求', async () => {
  const input = Readable.from(['{"ht', 'ml":"<svg></svg>"}'])
  assert.deepEqual(await readJSONStdin(input), { html: '<svg></svg>' })
})
