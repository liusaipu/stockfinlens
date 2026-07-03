#!/usr/bin/env python3
"""
Fetch RIM external data via akshare for A-share stocks.
Input: {"symbol": "300054"} via stdin
Output: JSON with eps_forecast, rf, beta, rm_rf, etc.
"""
import sys
import json
import math
import re
from datetime import datetime, timedelta

try:
    import akshare as ak
    import pandas as pd
    import numpy as np
except ImportError as ie:
    print(json.dumps({"error": f"missing dependency: {ie}"}), file=sys.stderr)
    sys.exit(1)


def _extract_eps_forecast(row) -> dict:
    """从一行 DataFrame 中提取预测每股收益，兼容 '2024预测每股收益' / '2024年预测每股收益' 等列名。"""
    forecast = {}
    for col in row.index:
        col_str = str(col)
        m = re.search(r"(\d{4})\s*年?\s*预测每股收益", col_str)
        if m:
            year = m.group(1)
            try:
                val = float(row[col])
                if not math.isnan(val) and val != 0:
                    forecast[year] = val
            except (ValueError, TypeError):
                pass
    return forecast


def _fetch_eps_forecast_em(symbol: str):
    """东方财富盈利预测：获取全市场数据后本地筛选目标股票。"""
    try:
        df = ak.stock_profit_forecast_em()
        if df is None or len(df) == 0:
            return None, "empty response from stock_profit_forecast_em"
        if "代码" not in df.columns:
            return None, "missing 代码 column in profit forecast"
        code_col = df["代码"].astype(str).str.strip()
        # 同时匹配 "000970" / "000970.SZ" / "000970.SH"
        matches = (code_col == symbol) | code_col.str.startswith(f"{symbol}.")
        row = df[matches]
        if len(row) > 0:
            return _extract_eps_forecast(row.iloc[0]), None
        return None, f"symbol {symbol} not found in profit forecast, rows={len(df)}, samples={list(code_col.head(3))}"
    except Exception as e:
        return None, str(e)


def _parse_number(val):
    """解析可能带 % 或中文单位的数值字符串。"""
    if val is None:
        return 0.0
    s = str(val).strip().replace(",", "")
    if s.endswith("%"):
        try:
            return float(s[:-1]) / 100
        except (ValueError, TypeError):
            return 0.0
    try:
        return float(s)
    except (ValueError, TypeError):
        return 0.0


def _fetch_eps_forecast_ths(symbol: str):
    """同花顺盈利预测。该接口的'业绩预测详表-详细指标预测'没有直接 EPS，但有每股净资产和净资产收益率，可推导 EPS = BPS * ROE。"""
    try:
        df = ak.stock_profit_forecast_ths(symbol=symbol, indicator="业绩预测详表-详细指标预测")
        if df is None or len(df) == 0:
            return None, "empty response from stock_profit_forecast_ths"

        # 查找每股净资产(BPS)和净资产收益率(ROE)行
        bps_row = None
        roe_row = None
        indicators = []
        for _, row in df.iterrows():
            indicator_name = str(row.get("预测指标", ""))
            indicators.append(indicator_name)
            if "每股净资产" in indicator_name:
                bps_row = row
            elif "净资产收益率" in indicator_name and "净资产收益率" not in indicator_name.replace("净资产收益率", ""):
                roe_row = row
            elif indicator_name.strip() == "净资产收益率":
                roe_row = row

        if bps_row is None or roe_row is None:
            return None, f"ths missing bps or roe row, indicators={indicators}"

        forecast = {}
        for col in df.columns:
            col_str = str(col)
            m = re.search(r"预测(\d{4})-平均", col_str)
            if m:
                year = m.group(1)
                bps_val = _parse_number(bps_row[col])
                roe_val = _parse_number(roe_row[col])
                if bps_val > 0 and roe_val > 0:
                    forecast[year] = bps_val * roe_val

        if forecast:
            return forecast, None
        return None, f"no eps forecast derived from ths bps*roe, bps={bps_row.to_dict()}, roe={roe_row.to_dict()}"
    except Exception as e:
        return None, str(e)


def _select_eps_forecast(em_forecast, em_err, ths_forecast, ths_err):
    """
    从东财和同花顺的 EPS 预测中选择更优数据。
    策略：优先预测年数更多的；年数相同时优先同花顺（用户验证过与同花顺一致）。
    """
    em_count = len(em_forecast) if em_forecast else 0
    ths_count = len(ths_forecast) if ths_forecast else 0

    if em_count == 0 and ths_count == 0:
        err_parts = []
        if em_err:
            err_parts.append(f"eastmoney: {em_err}")
        if ths_err:
            err_parts.append(f"ths: {ths_err}")
        return None, None, "; ".join(err_parts) if err_parts else "no forecast from any source"

    if ths_count >= em_count:
        return ths_forecast, "ths", None
    return em_forecast, "eastmoney", None


def fetch(symbol: str):
    result = {"symbol": symbol, "fetch_time": datetime.now().isoformat()}
    today = datetime.now()
    start_dt = today - timedelta(days=365)
    start_date = start_dt.strftime("%Y%m%d")
    end_date = today.strftime("%Y%m%d")

    # 1. EPS forecast: 同时获取东财和同花顺，选择更优数据
    em_forecast, em_err = _fetch_eps_forecast_em(symbol)
    ths_forecast, ths_err = _fetch_eps_forecast_ths(symbol)
    eps_forecast, eps_source, eps_err = _select_eps_forecast(em_forecast, em_err, ths_forecast, ths_err)

    if eps_forecast:
        result["eps_forecast"] = eps_forecast
        result["eps_forecast_source"] = eps_source
        result["eps_forecast_count"] = len(eps_forecast)
        result["eps_forecast_em_count"] = len(em_forecast) if em_forecast else 0
        result["eps_forecast_ths_count"] = len(ths_forecast) if ths_forecast else 0
    else:
        result["eps_forecast"] = None
        result["eps_forecast_error"] = eps_err or "unknown error"
        result["eps_forecast_source"] = None
        result["eps_forecast_count"] = 0
        result["eps_forecast_em_count"] = len(em_forecast) if em_forecast else 0
        result["eps_forecast_ths_count"] = len(ths_forecast) if ths_forecast else 0

    # 2. Risk-free rate (China 10-year bond yield)
    rf = 0.0
    rf_date = ""
    try:
        bond_df = ak.bond_zh_yield(symbol="国债收益率10年", period="日", start_date="", end_date="")
        if len(bond_df) > 0:
            rf = float(bond_df.iloc[-1]["收盘价"]) / 100
            rf_date = str(bond_df.index[-1]) if hasattr(bond_df, "index") else str(bond_df.iloc[-1].get("日期", ""))
    except Exception:
        pass
    if rf <= 0:
        try:
            bond_df = ak.bond_zh_us_rate()
            rf_row = bond_df.dropna(subset=["中国国债收益率10年"]).iloc[-1]
            rf = float(rf_row["中国国债收益率10年"]) / 100
            rf_date = str(rf_row["日期"])
        except Exception as e:
            result["rf_error"] = str(e)
    if rf <= 0:
        rf = 0.0183
        rf_date = ""
    result["rf"] = rf
    result["rf_date"] = rf_date

    # 3. Basic info for shares / price / pb
    try:
        info_df = ak.stock_individual_info_em(symbol=symbol)
        info = dict(zip(info_df["item"], info_df["value"]))
        result["price"] = float(info.get("最新", 0))
        result["total_shares"] = float(info.get("总股本", 0))
        result["market_cap"] = float(info.get("总市值", 0))
    except Exception as e:
        result["info_error"] = str(e)
        result["price"] = 0
        result["total_shares"] = 0
        result["market_cap"] = 0

    # 4. PB from daily indicator
    pb = 0.0
    try:
        df_pb = ak.stock_zh_a_spot_em()
        row_pb = df_pb[df_pb["代码"] == symbol]
        if len(row_pb) > 0:
            pb = float(row_pb.iloc[0].get("市净率", 0))
    except Exception:
        pass
    result["pb"] = pb if pb > 0 else 0

    # 5. Beta: 个股日收益率 vs 沪深300 日收益率，近1年
    beta = 0.98
    beta_date = ""
    try:
        stock_df = ak.stock_zh_a_hist(symbol=symbol, period="daily", start_date=start_date, end_date=end_date, adjust="hfq")
        index_df = ak.index_zh_a_hist(symbol="000300", period="daily", start_date=start_date, end_date=end_date)
        if len(stock_df) > 30 and len(index_df) > 30:
            stock_df["ret"] = stock_df["收盘"].pct_change()
            index_df["ret"] = index_df["收盘"].pct_change()
            merged = pd.merge(stock_df[["日期", "ret"]], index_df[["日期", "ret"]], on="日期", suffixes=("_s", "_m")).dropna()
            if len(merged) > 30:
                stock_ret = merged["ret_s"].values
                market_ret = merged["ret_m"].values
                cov = np.cov(stock_ret, market_ret)[0, 1]
                var = np.var(market_ret)
                if var > 0:
                    beta = float(cov / var)
                    beta_date = str(merged["日期"].iloc[-1])
    except Exception as e:
        result["beta_error"] = str(e)
    result["beta"] = beta
    result["beta_date"] = beta_date

    # 6. Market risk premium (Rm-Rf): 用沪深300 PE 倒数作为盈利收益率，再减去 Rf 得到隐含风险溢价
    rm_rf = 0.0517
    rm_rf_date = ""
    try:
        pe_df = ak.index_value_name_funddb(name="沪深300")
        if len(pe_df) > 0:
            last = pe_df.iloc[-1]
            pe = float(last.get("市盈率", 0))
            if pe > 0:
                # 隐含股权风险溢价 = 盈利收益率 - 无风险利率
                rm_rf = max(0.02, (1.0 / pe) - rf)
                rm_rf_date = str(last.get("日期", ""))
    except Exception as e:
        result["rm_rf_error"] = str(e)
    result["rm_rf"] = rm_rf
    result["rm_rf_date"] = rm_rf_date

    return result


def main():
    req = json.load(sys.stdin)
    symbol = req.get("symbol", "")
    if not symbol:
        print(json.dumps({"error": "symbol is required"}))
        return
    try:
        resp = fetch(symbol)
        print(json.dumps(resp, ensure_ascii=False, default=str))
    except Exception as e:
        print(json.dumps({"error": str(e)}))


if __name__ == "__main__":
    main()
