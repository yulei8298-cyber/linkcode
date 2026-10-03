// 图表配色（设计稿 A）：第一序列用品牌橙，其余用低饱和色区分，深浅主题各一套。
// 文字与网格颜色与 lc-tokens 中的 ink-2 / line 对应。

export interface ChartPalette {
  text: string
  grid: string
  series: string[]
}

const LIGHT: ChartPalette = {
  text: '#57574f',
  grid: '#e3e3de',
  series: ['#d4561b', '#57574f', '#4f6fd0', '#2f7a50', '#a8781f', '#8a5cb8'],
}

const DARK: ChartPalette = {
  text: '#a6a6a1',
  grid: '#26262a',
  series: ['#ff8a4c', '#cfcfca', '#7d97e8', '#6fc08f', '#e0a85a', '#b08ad8'],
}

export function chartPalette(dark: boolean): ChartPalette {
  return dark ? DARK : LIGHT
}

// 给 #rrggbb 加透明度，用于面积填充
export function withAlpha(hex: string, alpha: number): string {
  const value = Math.round(Math.min(1, Math.max(0, alpha)) * 255)
    .toString(16)
    .padStart(2, '0')
  return `${hex}${value}`
}
