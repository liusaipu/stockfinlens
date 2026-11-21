interface RIMForecastInput {
  EPS?: number[] | null
  Years?: string[] | null
}

export function prepareRIMModalForecast(forecast?: RIMForecastInput, currentYear = new Date().getFullYear()) {
  const rawEPS = forecast?.EPS ?? []
  const rawYears = forecast?.Years ?? []
  const entries = rawEPS.slice(0, 6).map((value, index) => ({
    value,
    year: rawYears[index] ?? '',
  })).filter(({ year }) => {
    if (!year) return true
    const parsed = Number(year)
    return !Number.isFinite(parsed) || parsed >= currentYear
  })

  const eps = entries.map(({ value }) => Number(value).toFixed(2))
  const years = entries.map(({ year }) => year)
  while (eps.length < 6) {
    eps.push('0.00')
    years.push(String(currentYear + years.length))
  }
  return { eps, years }
}

export function rimEPSSourceLabel(source?: string) {
  switch (source) {
    case 'ths':
      return '同花顺机构一致预期'
    case 'eastmoney':
      return '东方财富机构一致预期'
    case 'manual':
      return '手动输入'
    default:
      return '机构一致预期'
  }
}
