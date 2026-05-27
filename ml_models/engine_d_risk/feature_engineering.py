#!/usr/bin/env python3
"""
Engine-D 特征工程
从 FinancialData 提取 26 维风险预警特征

用法:
  from feature_engineering import extract_engine_d_features
  features = extract_engine_d_features(financial_data, market_data, profile_data)
"""

import json
import math
from typing import Dict, List, Optional, Any


def safe_div(a: float, b: float, default: float = 0.0) -> float:
    """安全除法"""
    if b == 0 or math.isnan(b):
        return default
    return a / b


def safe_get(data: Dict, key: str, default: float = 0.0) -> float:
    """安全获取数值"""
    val = data.get(key, default)
    if val is None or math.isnan(val):
        return default
    return float(val)


def extract_financial_features(fin_data: Dict[str, Any], year: str) -> Dict[str, float]:
    """
    从财务数据提取 Engine-D 财务特征 (14维)
    
    Args:
        fin_data: FinancialData 结构
        year: 年份，如 "2023"
    
    Returns:
        14维财务特征字典
    """
    if not fin_data or year not in fin_data.get('Years', []):
        return {}
    
    bs = fin_data.get('BalanceSheet', {}).get(year, {})
    inc = fin_data.get('IncomeStatement', {}).get(year, {})
    cf = fin_data.get('CashFlow', {}).get(year, {})
    
    if not bs or not inc or not cf:
        return {}
    
    # 基础数据
    total_assets = safe_get(bs, '总资产')
    total_liabilities = safe_get(bs, '总负债')
    equity = safe_get(bs, '所有者权益合计')
    if equity == 0:
        equity = total_assets - total_liabilities
    
    revenue = safe_get(inc, '营业收入')
    cost = safe_get(inc, '营业成本')
    net_profit = safe_get(inc, '净利润')
    operating_profit = safe_get(inc, '营业利润')
    
    op_cash_flow = safe_get(cf, '经营活动产生的现金流量净额')
    
    inventory = safe_get(bs, '存货')
    receivables = safe_get(bs, '应收账款')
    notes_receivable = safe_get(bs, '应收票据')
    total_receivables = receivables + notes_receivable
    
    goodwill = safe_get(bs, '商誉')
    
    # 1. M-Score (Beneish) - 简化计算
    # 实际应该使用跨年度数据计算完整 M-Score
    # 这里使用应计项目作为代理
    accruals = net_profit - op_cash_flow
    mscore = -safe_div(accruals, total_assets, 0) * 5  # 简化为负的应计比例
    
    # 2. Z-Score (Altman) - 简化版
    # Z = 1.2*X1 + 1.4*X2 + 3.3*X3 + 0.6*X4 + 1.0*X5
    working_capital = safe_get(bs, '流动资产合计') - safe_get(bs, '流动负债合计')
    x1 = safe_div(working_capital, total_assets, 0)
    retained_earnings = safe_get(bs, '未分配利润')
    x2 = safe_div(retained_earnings, total_assets, 0)
    ebit = operating_profit + safe_get(inc, '财务费用')
    x3 = safe_div(ebit, total_assets, 0)
    # X4 (市值/负债) 需要市场数据，这里使用账面价值替代
    x4 = safe_div(equity, total_liabilities, 1) if total_liabilities > 0 else 1
    x5 = safe_div(revenue, total_assets, 0)
    
    zscore = 1.2 * x1 + 1.4 * x2 + 3.3 * x3 + 0.6 * x4 + 1.0 * x5
    
    # 3. 现金流偏离度 (净利润 - 经营现金流) / 总资产
    cash_deviation = safe_div(abs(accruals), total_assets, 0)
    
    # 4. 应收账款异常度 = 应收账款 / 营收
    ar_risk = safe_div(total_receivables, revenue, 0)
    
    # 5. 毛利率风险（单边）：离线特征提取拿不到上一年数据，置 0
    #    运行时由 Go 端 BuildMLEngineDInput 基于 step8 给出真实跌幅
    gross_margin = safe_div(revenue - cost, revenue, 0)
    gm_risk = 0.0

    # 6. A-Score 综合风险：离线提取使用简化代理；运行时由 Go 端覆盖为真实 A-Score
    ascore = 0
    if mscore > -2.22:
        ascore += 20
    if zscore < 1.81:
        ascore += 25
    if cash_deviation > 0.2:
        ascore += 15
    if ar_risk > 0.3:
        ascore += 15
    if safe_div(total_liabilities, total_assets, 0) > 0.7:
        ascore += 15
    if goodwill > equity * 0.5:
        ascore += 10
    ascore = min(ascore, 100)
    
    # 7. ROE
    roe = safe_div(net_profit, equity, 0)
    
    # 8. 营收增长率（需要前一年数据）
    prev_year = str(int(year) - 1)
    prev_revenue = 0
    if prev_year in fin_data.get('IncomeStatement', {}):
        prev_revenue = safe_get(fin_data['IncomeStatement'][prev_year], '营业收入')
    revenue_growth = safe_div(revenue - prev_revenue, prev_revenue, 0) if prev_revenue > 0 else 0
    
    # 9. 资产负债率
    debt_ratio = safe_div(total_liabilities, total_assets, 0)
    
    # 10. 净利润现金含量
    ncf_to_profit = safe_div(op_cash_flow, net_profit, 0) if net_profit > 0 else 0
    
    # 11. 商誉/净资产
    goodwill_to_equity = safe_div(goodwill, equity, 0)
    
    # 12. 存货周转率
    cogs = cost  # 简化使用营业成本
    avg_inventory = inventory  # 简化使用期末存货
    inventory_turnover = safe_div(cogs, avg_inventory, 0)
    
    # 13. 应收账款周转率
    avg_receivables = total_receivables
    receivable_turnover = safe_div(revenue, avg_receivables, 0)
    
    return {
        'mscore': mscore,
        'zscore': zscore,
        'cash_deviation': cash_deviation,
        'ar_risk': ar_risk,
        'gm_risk': gm_risk,
        'ascore': ascore,
        'roe': roe,
        'gross_margin': gross_margin,
        'revenue_growth': revenue_growth,
        'debt_ratio': debt_ratio,
        'ncf_to_profit': ncf_to_profit,
        'goodwill_to_equity': goodwill_to_equity,
        'inventory_turnover': inventory_turnover,
        'receivable_turnover': receivable_turnover,
    }


def extract_market_features(market_data: Dict[str, Any]) -> Dict[str, float]:
    """
    提取市场特征 (6维)
    
    Args:
        market_data: 市场数据，包含行情、估值等信息
    
    Returns:
        6维市场特征字典
    """
    if not market_data:
        return {
            'pe_ttm': 0, 'pb': 0, 'market_cap': 0,
            'turnover_20d': 0, 'volatility_60d': 0, 'max_drawdown_1y': 0
        }
    
    # 1. PE_TTM
    pe_ttm = market_data.get('pe_ttm', 0) or market_data.get('pe', 0)
    if pe_ttm is None or pe_ttm > 1000 or pe_ttm < 0:
        pe_ttm = 0
    
    # 2. PB
    pb = market_data.get('pb', 0) or market_data.get('pb_ratio', 0)
    if pb is None or pb > 100:
        pb = 0
    
    # 3. 市值（亿元）
    market_cap = market_data.get('market_cap', 0)
    if market_cap > 1e8:  # 如果是元，转换为亿元
        market_cap = market_cap / 1e8
    
    # 4. 20日换手率
    turnover_20d = market_data.get('turnover_20d', 0.03)
    
    # 5. 60日波动率
    volatility_60d = market_data.get('volatility_60d', 0.3)
    
    # 6. 近1年最大回撤
    max_drawdown_1y = market_data.get('max_drawdown_1y', -0.2)
    
    return {
        'pe_ttm': float(pe_ttm or 0),
        'pb': float(pb or 0),
        'market_cap': float(market_cap or 0),
        'turnover_20d': float(turnover_20d),
        'volatility_60d': float(volatility_60d),
        'max_drawdown_1y': float(max_drawdown_1y),
    }


def extract_non_financial_features(profile_data: Dict[str, Any]) -> Dict[str, float]:
    """
    提取非财务特征 (6维)
    
    Args:
        profile_data: 公司资料数据
    
    Returns:
        6维非财务特征字典
    """
    if not profile_data:
        return {
            'pledge_ratio': 0, 'regulatory_inquiry_count_1y': 0,
            'major_shareholder_reduction_1y': 0, 'auditor_switch_count_2y': 0,
            'cfo_change_count_2y': 0
        }
    
    # 这些特征需要外部数据源，这里使用默认值或从已有数据中提取
    risk_profile = profile_data.get('risk_profile', {})
    
    return {
        'pledge_ratio': risk_profile.get('pledge_ratio', 0),
        'regulatory_inquiry_count_1y': risk_profile.get('regulatory_inquiry_count_1y', 0),
        'major_shareholder_reduction_1y': risk_profile.get('major_shareholder_reduction_1y', 0),
        'auditor_switch_count_2y': risk_profile.get('auditor_switch_count_2y', 0),
        'cfo_change_count_2y': risk_profile.get('cfo_change_count_2y', 0),
    }


def extract_engine_d_features(
    fin_data: Dict[str, Any],
    market_data: Optional[Dict[str, Any]] = None,
    profile_data: Optional[Dict[str, Any]] = None,
    year: Optional[str] = None
) -> List[float]:
    """
    提取完整的 Engine-D 26维特征向量
    
    Args:
        fin_data: 财务数据
        market_data: 市场数据（可选）
        profile_data: 公司资料（可选）
        year: 年份，默认使用最新年份
    
    Returns:
        26维特征向量列表
    """
    if not fin_data:
        return [0.0] * 26
    
    # 确定年份
    if year is None:
        years = fin_data.get('Years', [])
        if not years:
            return [0.0] * 26
        year = years[0]  # 使用第一个（最新）年份
    
    # 提取各维度特征
    fin_features = extract_financial_features(fin_data, year)
    market_features = extract_market_features(market_data or {})
    non_fin_features = extract_non_financial_features(profile_data or {})
    
    # 合并为 26 维向量
    feature_order = [
        # 财务指标 (14)
        'mscore', 'zscore', 'cash_deviation', 'ar_risk', 'gm_risk', 'ascore',
        'roe', 'gross_margin', 'revenue_growth', 'debt_ratio', 'ncf_to_profit',
        'goodwill_to_equity', 'inventory_turnover', 'receivable_turnover',
        # 市场指标 (6)
        'pe_ttm', 'pb', 'market_cap', 'turnover_20d', 'volatility_60d', 'max_drawdown_1y',
        # 非财务指标 (6)
        'pledge_ratio', 'regulatory_inquiry_count_1y', 'major_shareholder_reduction_1y',
        'auditor_switch_count_2y', 'cfo_change_count_2y'
    ]
    
    all_features = {**fin_features, **market_features, **non_fin_features}
    
    return [float(all_features.get(k, 0.0)) for k in feature_order]


def extract_engine_d_features_dict(
    fin_data: Dict[str, Any],
    market_data: Optional[Dict[str, Any]] = None,
    profile_data: Optional[Dict[str, Any]] = None,
    year: Optional[str] = None
) -> Dict[str, float]:
    """
    提取特征为字典格式（便于调试）
    """
    if not fin_data:
        return {}
    
    if year is None:
        years = fin_data.get('Years', [])
        if not years:
            return {}
        year = years[0]
    
    fin_features = extract_financial_features(fin_data, year)
    market_features = extract_market_features(market_data or {})
    non_fin_features = extract_non_financial_features(profile_data or {})
    
    return {
        **{f"fin_{k}": v for k, v in fin_features.items()},
        **{f"mkt_{k}": v for k, v in market_features.items()},
        **{f"nonfin_{k}": v for k, v in non_fin_features.items()},
    }


if __name__ == '__main__':
    # 测试代码
    print("Engine-D 特征工程模块")
    print("=" * 60)
    print("财务特征 (14维):", list(extract_financial_features({}, "2023").keys()))
    print("市场特征 (6维):", list(extract_market_features({}).keys()))
    print("非财务特征 (6维):", list(extract_non_financial_features({}).keys()))
    print("=" * 60)
    print("总特征维度:", 26)
