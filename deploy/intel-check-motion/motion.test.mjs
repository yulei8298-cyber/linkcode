import assert from 'node:assert/strict'
import test from 'node:test'
import { analyzeMotionSamples } from './motion.mjs'

function sample(angle) {
  const crank = { x: 100, y: 100 }
  const point = (phase) => ({ x: crank.x + Math.cos(angle + phase) * 30, y: crank.y + Math.sin(angle + phase) * 30 })
  const left = point(0)
  const right = point(Math.PI)
  return { parts: {
    'wheel-rear': { center: { x: 40, y: 120 }, matrix: { a: Math.cos(angle), b: Math.sin(angle), c: -Math.sin(angle), d: Math.cos(angle) } },
    'wheel-front': { center: { x: 180, y: 120 }, matrix: { a: Math.cos(angle), b: Math.sin(angle), c: -Math.sin(angle), d: Math.cos(angle) } },
    'crank-center': { center: crank, matrix: { a: 1, b: 0, c: 0, d: 1 } },
    'pedal-left': { center: left, matrix: { a: 1, b: 0, c: 0, d: 1 } },
    'pedal-right': { center: right, matrix: { a: 1, b: 0, c: 0, d: 1 } },
    'foot-left': { center: left, matrix: { a: 1, b: 0, c: 0, d: 1 } },
    'foot-right': { center: right, matrix: { a: 1, b: 0, c: 0, d: 1 } },
    'leg-left': { center: left, matrix: { a: 1, b: 0, c: 0, d: 1 }, endpoints: [{ x: 80, y: 60 }, left] },
    'leg-right': { center: right, matrix: { a: 1, b: 0, c: 0, d: 1 }, endpoints: [{ x: 120, y: 60 }, right] },
  } }
}

test('正确的反相脚踏轨迹通过', () => {
  const samples = Array.from({ length: 16 }, (_, index) => sample(index * 0.22))
  assert.equal(analyzeMotionSamples(samples).pass, true)
})

test('脚与踏板脱节时失败', () => {
  const samples = Array.from({ length: 16 }, (_, index) => {
    const current = sample(index * 0.22)
    current.parts['foot-left'].center = { x: 250, y: 250 }
    return current
  })
  const result = analyzeMotionSamples(samples)
  assert.equal(result.pass, false)
  assert.equal(result.checks.find((check) => check.item === 'left 脚跟随踏板').pass, false)
})
