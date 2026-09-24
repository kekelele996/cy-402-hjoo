// 将分钟数格式化为「x 小时 y 分钟」。
export function formatDuration(minutes?: number | null): string {
  const m = Number(minutes || 0)
  if (m <= 0) return '0 分钟'
  const h = Math.floor(m / 60)
  const rest = m % 60
  if (h === 0) return `${rest} 分钟`
  if (rest === 0) return `${h} 小时`
  return `${h} 小时 ${rest} 分钟`
}

// 将分钟数格式化为小时数（两位小数），用于金额核算展示。
export function formatHours(minutes?: number | null): string {
  return (Number(minutes || 0) / 60).toFixed(2)
}
