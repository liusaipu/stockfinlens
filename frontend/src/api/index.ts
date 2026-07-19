// 统一 barrel：调用方可以 `import { AnalyzeStock } from './api'` 而不必关心分桶。
// 同时也可以按需 `import { AnalyzeStock } from './api/analysis'` 减少包大小（虽然 Vite 默认 tree-shake）。

export * from './errors'
export * from './watchlist'
export * from './analysis'
export * from './data'
export * from './quotes'
export * from './profile'
export * from './report'
export * from './settings'
export * from './admin'
export * from './margin'
export * from './hotposts'
