// 自选股分组相关（分组 CRUD / 自动建议 / 板块热度 / 组内对比）
import { Go_ } from './wrap'

export const {
  GetWatchlistGroups,
  SaveWatchlistGroups,
  SuggestWatchlistGroups,
  GetGroupHeat,
  GetGroupComparison,
  FetchMissingCompositeData,
} = Go_
