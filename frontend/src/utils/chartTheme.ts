// 图表配色（设计稿 A）：第一序列用品牌橙，其余用低饱和色区分，深浅主题各一套。
// series 同时用作分类图（饼图、排行）的色板，前 12 个足够区分；超出部分循环使用。
// 文字与网格颜色与 lc-tokens 中的 ink-2 / line 对应。

export interface ChartPalette {
  text: string
  grid: string
  series: string[]
  /** 「其他」汇总项用的中性灰 */
  other: string
}

const LIGHT: ChartPalette = {
  text: '#57574f',
  grid: '#e3e3de',
  series: ['#d4561b', '#57574f', '#4f6fd0', '#2f7a50', '#a8781f', '#8a5cb8', '#2b8a9e', '#b5475a', '#5e8f2f', '#c08a62', '#6b5fa8', '#8c8c84'],
  other: '#cfcfc8',
}

const DARK: ChartPalette = {
  text: '#a6a6a1',
  grid: '#26262a',
  series: ['#ff8a4c', '#cfcfca', '#7d97e8', '#6fc08f', '#e0a85a', '#b08ad8', '#5fb8c9', '#e07a8c', '#9cc46a', '#d9a882', '#968be0', '#85857f'],
  other: '#38383b',
}

export function chartPalette(dark: boolean): ChartPalette {
  return dark ? DARK : LIGHT
}

export function seriesColors(dark: boolean, count: number): string[] {
  const { series } = chartPalette(dark)
  return Array.from({ length: count }, (_, i) => series[i % series.length])
}

// 给 #rrggbb 加透明度，用于面积填充
export function withAlpha(hex: string, alpha: number): string {
  const value = Math.round(Math.min(1, Math.max(0, alpha)) * 255)
    .toString(16)
    .padStart(2, '0')
  return `${hex}${value}`
}
