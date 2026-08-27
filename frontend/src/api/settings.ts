// 应用设置 / 自动更新 / 风险敏感度 / StockFinLens 数据源
import { Go_ } from './wrap'

export const {
  GetSFLConfig,
  SaveSFLConfig,
  VerifySFLToken,
  GetAutoCheckUpdate,
  SetAutoCheckUpdate,
  GetRiskSensitivity,
  SetRiskSensitivity,
  GetAIConfig,
  SaveAIConfig,
  TestAIConnection,
  ListLLMModels,
  AnalyzeStockWithAI,
  LoadAIResearchReport,
  CancelAIResearch,
  CheckForUpdate,
  SkipVersion,
  DownloadUpdate,
  ApplyUpdate,
  GetCurrentVersion,
  GetProxyConfig,
  SetProxyConfig,
} = Go_
