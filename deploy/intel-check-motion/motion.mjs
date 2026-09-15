export const REQUIRED_PARTS = [
  'wheel-rear',
  'wheel-front',
  'crank-center',
  'pedal-left',
  'pedal-right',
  'foot-left',
  'foot-right',
  'leg-left',
  'leg-right',
]

const distance = (a, b) => Math.hypot(a.x - b.x, a.y - b.y)
const round = (value) => Math.round(value * 100) / 100

function maximumDrift(points) {
  if (points.length < 2) return 0
  const origin = points[0]
  return Math.max(...points.map((point) => distance(point, origin)))
}

function pathLength(points) {
  let total = 0
  for (let index = 1; index < points.length; index += 1) {
    total += distance(points[index - 1], points[index])
  }
  return total
}

function unwrapTravel(angles) {
  let total = 0
  for (let index = 1; index < angles.length; index += 1) {
    let delta = angles[index] - angles[index - 1]
    while (delta > Math.PI) delta -= Math.PI * 2
    while (delta < -Math.PI) delta += Math.PI * 2
    total += Math.abs(delta)
  }
  return total
}

function phaseError(leftAngles, rightAngles) {
  const errors = leftAngles.map((left, index) => {
    let delta = Math.abs(left - rightAngles[index]) % (Math.PI * 2)
    if (delta > Math.PI) delta = Math.PI * 2 - delta
    return Math.abs(Math.PI - delta)
  })
  return errors.reduce((sum, value) => sum + value, 0) / errors.length
}

function rotationChange(matrices) {
  if (matrices.length < 2) return 0
  const first = matrices[0]
  return Math.max(...matrices.map((matrix) => Math.hypot(
    matrix.a - first.a,
    matrix.b - first.b,
    matrix.c - first.c,
    matrix.d - first.d,
  )))
}

export function analyzeMotionSamples(samples) {
  const checks = []
  const add = (item, pass, value, limit, detail) => {
    checks.push({ item, pass, value: round(value), limit, detail })
  }
  const positions = (part) => samples.map((sample) => sample.parts[part].center)
  const matrices = (part) => samples.map((sample) => sample.parts[part].matrix)

  for (const part of ['wheel-rear', 'wheel-front']) {
    const drift = maximumDrift(positions(part))
    add(`${part} 轮心稳定`, drift <= 4, drift, 4,
      `轮心最大漂移 ${round(drift)}px，要求不超过 4px`)
    const change = rotationChange(matrices(part))
    add(`${part} 持续旋转`, change >= 0.08, change, 0.08,
      `旋转矩阵变化量 ${round(change)}，要求至少 0.08`)
  }

  const crank = positions('crank-center')
  const crankDrift = maximumDrift(crank)
  add('曲柄轴心稳定', crankDrift <= 4, crankDrift, 4,
    `曲柄轴心最大漂移 ${round(crankDrift)}px，要求不超过 4px`)

  const pedalAngles = {}
  for (const side of ['left', 'right']) {
    const pedal = positions(`pedal-${side}`)
    const radii = pedal.map((point, index) => distance(point, crank[index]))
    const meanRadius = radii.reduce((sum, value) => sum + value, 0) / radii.length
    const radiusSpread = Math.max(...radii) - Math.min(...radii)
    add(`${side} 踏板绕轴`, meanRadius >= 5 && radiusSpread <= Math.max(6, meanRadius * 0.35),
      radiusSpread, round(Math.max(6, meanRadius * 0.35)),
      `平均半径 ${round(meanRadius)}px，半径波动 ${round(radiusSpread)}px`)

    const angles = pedal.map((point, index) => Math.atan2(point.y - crank[index].y, point.x - crank[index].x))
    pedalAngles[side] = angles
    const travel = unwrapTravel(angles)
    add(`${side} 踏板持续运动`, travel >= 1.5, travel, 1.5,
      `采样期间角位移 ${round(travel)}rad，要求至少 1.5rad`)

    const foot = positions(`foot-${side}`)
    const footDistances = foot.map((point, index) => distance(point, pedal[index]))
    const maxFootDistance = Math.max(...footDistances)
    add(`${side} 脚跟随踏板`, maxFootDistance <= 32, maxFootDistance, 32,
      `脚与踏板中心最大距离 ${round(maxFootDistance)}px，要求不超过 32px`)

    const legDistances = samples.map((sample) => sample.parts[`leg-${side}`].endpoints
      .reduce((best, point) => Math.min(best, distance(point, sample.parts[`foot-${side}`].center)), Infinity))
    const maxLegDistance = Math.max(...legDistances)
    add(`${side} 腿脚连续`, maxLegDistance <= 28, maxLegDistance, 28,
      `腿端与脚中心最大距离 ${round(maxLegDistance)}px，要求不超过 28px`)
  }

  const phase = phaseError(pedalAngles.left, pedalAngles.right)
  add('双踏板反相', phase <= 0.65, phase, 0.65,
    `双踏板与 180° 反相的平均误差 ${round(phase)}rad，要求不超过 0.65rad`)

  const pass = checks.every((check) => check.pass)
  return {
    version: 'browser_motion_v1',
    verifiable: true,
    pass,
    reason: pass ? '部件标记完整，结构与采样轨迹均达到门禁要求' : '部件标记完整，但一项或多项采样轨迹未达到门禁要求',
    checks,
  }
}

export function unverified(reason) {
  return { version: 'browser_motion_v1', verifiable: false, pass: false, reason, checks: [] }
}
