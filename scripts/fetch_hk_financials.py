#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
获取港股财务数据（资产负债表、利润表、现金流量表）
用法: python3 fetch_hk_financials.py <code>
输出 JSON
"""
import json
import sys
import io
import os
import contextlib
import pandas as pd

# 强制 stdout 使用 UTF-8，避免 Windows 下 GBK 编码导致中文乱码
if sys.platform == "win32":
    sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8")
    os.environ["PYTHONIOENCODING"] = "utf-8"

os.environ.setdefault("TQDM_DISABLE", "1")
os.environ.setdefault("PYTHONUNBUFFERED", "1")


try:
    import akshare as ak
except ImportError:
    ak = None


# 把第三方库可能输出到 stdout 的警告/进度条重定向到 stderr，保证 stdout 只有最终 JSON
@contextlib.contextmanager
def _suppress_stdout():
    old_stdout = sys.stdout
    sys.stdout = sys.stderr
    try:
        yield
    finally:
        sys.stdout = old_stdout


# 港股财报科目名 -> A股分析器使用的标准名称映射（支持一对多）
ITEM_NAME_MAP = {
    # 资产负债表
    "总资产": ["资产合计"],
    "总负债": ["负债合计"],
    "股东权益": ["所有者权益合计", "归母所有者权益合计"],
    "总权益": ["所有者权益合计", "归母所有者权益合计"],
    "净资产": ["所有者权益合计", "归母所有者权益合计"],
    "流动资产合计": ["流动资产合计"],
    "流动负债合计": ["流动负债合计"],
    "现金及等价物": ["货币资金"],
    "存货": ["存货"],
    "应收帐款": ["应收账款"],
    "应收票据": ["应收票据"],
    "预付款按金及其他应收款": ["预付款项"],
    "预付款项": ["预付款项"],
    "应付帐款": ["应付账款"],
    "应付票据": ["应付票据"],
    "预收款项": ["预收款项"],
    "其他应付款及应计费用": ["其他应付款"],
    "物业厂房及设备": ["固定资产"],
    "在建工程": ["在建工程"],
    "工程物资": ["工程物资"],
    "无形资产": ["无形资产"],
    "商誉": ["商誉"],
    "长期投资": ["长期股权投资"],
    "其他投资": ["其他权益工具投资"],
    "指定以公允价值记账之金融资产": ["可供出售金融资产"],
    "指定以公允价值记账之金融资产(流动)": ["交易性金融资产"],
    "短期投资": ["交易性金融资产"],
    "递延税项资产": ["递延所得税资产"],
    "短期贷款": ["短期借款"],
    "长期贷款": ["长期借款"],
    "一年内到期的非流动负债": ["一年内到期的非流动负债"],
    "应付债券": ["应付债券"],
    "租赁负债": ["租赁负债"],
    "长期应付款": ["长期应付款"],
    "递延税项负债": ["递延所得税负债"],
    "其他非流动负债": ["其他非流动负债"],
    "少数股东权益": ["少数股东权益"],
    "股本": ["股本"],
    "储备": ["未分配利润"],
    # 利润表
    "营业额": ["营业收入"],
    "营运收入": ["营业收入"],
    "销售成本": ["营业成本"],
    "毛利": ["毛利润"],
    "其他收入": ["其他收益"],
    "其他收益": ["营业外收入"],
    "销售及分销费用": ["销售费用"],
    "行政开支": ["管理费用"],
    "研发费用": ["研发费用"],
    "融资成本": ["财务费用"],
    "经营溢利": ["营业利润"],
    "除税前溢利": ["利润总额"],
    "税项": ["所得税费用"],
    "除税后溢利": ["净利润"],
    "股东应占溢利": ["归母净利润", "净利润"],
    "少数股东损益": ["少数股东损益"],
    "每股基本盈利": ["基本每股收益"],
    "每股摊薄盈利": ["稀释每股收益"],
    # 现金流量表
    "经营业务现金净额": ["经营活动现金流量净额"],
    "投资业务现金净额": ["投资活动现金流量净额"],
    "融资业务现金净额": ["筹资活动现金流量净额"],
    "现金净额": ["现金及现金等价物净增加额"],
    "期末现金": ["期末现金及现金等价物余额"],
}


def fetch_report_sheet(code, sheet_name):
    """获取指定报表，返回 DataFrame"""
    try:
        df = ak.stock_financial_hk_report_em(stock=code, symbol=sheet_name, indicator="年报")
        if df is not None and not df.empty:
            return df
    except Exception as e:
        return None
    return None


def parse_sheet(df):
    """解析报表 DataFrame 为 {item_name: {year: amount}}（与 A 股结构一致）"""
    result = {}
    if df is None or df.empty:
        return result
    df = df.copy()
    df["REPORT_DATE"] = pd.to_datetime(df["REPORT_DATE"], errors="coerce")
    df["YEAR"] = df["REPORT_DATE"].dt.year.astype(str)
    for _, row in df.iterrows():
        name = str(row.get("STD_ITEM_NAME", "")).strip()
        year = str(row.get("YEAR", ""))
        amount = row.get("AMOUNT")
        if name and year and pd.notna(amount):
            if name not in result:
                result[name] = {}
            result[name][year] = float(amount)
            # 同时写入映射后的标准名称
            for mapped in ITEM_NAME_MAP.get(name, []):
                if mapped and mapped != name:
                    if mapped not in result:
                        result[mapped] = {}
                    result[mapped][year] = float(amount)
    return result


def add_combined_items(sheet):
    """补充 A 股常见的合并科目（港股可能拆分为多个科目）"""
    # 应付票据及应付账款 = 应付账款 + 应付票据
    if "应付账款" in sheet or "应付票据" in sheet:
        years = set()
        if "应付账款" in sheet:
            years.update(sheet["应付账款"].keys())
        if "应付票据" in sheet:
            years.update(sheet["应付票据"].keys())
        sheet["应付票据及应付账款"] = {}
        for y in years:
            v = sheet.get("应付账款", {}).get(y, 0) + sheet.get("应付票据", {}).get(y, 0)
            if v != 0:
                sheet["应付票据及应付账款"][y] = v

    # 应收票据及应收账款 = 应收账款 + 应收票据
    if "应收账款" in sheet or "应收票据" in sheet:
        years = set()
        if "应收账款" in sheet:
            years.update(sheet["应收账款"].keys())
        if "应收票据" in sheet:
            years.update(sheet["应收票据"].keys())
        sheet["应收票据及应收账款"] = {}
        for y in years:
            v = sheet.get("应收账款", {}).get(y, 0) + sheet.get("应收票据", {}).get(y, 0)
            if v != 0:
                sheet["应收票据及应收账款"][y] = v

    return sheet


def fetch_analysis_indicators(code):
    """获取分析指标，补充 ROE、毛利率、营业收入、净利润等关键指标"""
    try:
        df = ak.stock_financial_hk_analysis_indicator_em(symbol=code, indicator="年报")
        if df is None or df.empty:
            return {}
        df = df.copy()
        df["REPORT_DATE"] = pd.to_datetime(df.get("REPORT_DATE", pd.Series()), errors="coerce")
        df["YEAR"] = df["REPORT_DATE"].dt.year.astype(str)
        result = {}
        indicator_map = {
            "ROE_AVG": "净资产收益率",
            "ROE_YEARLY": "净资产收益率",
            "GROSS_PROFIT_RATIO": "毛利率",
            "OPERATE_INCOME": "营业收入",
            "HOLDER_PROFIT": "净利润",
            "BASIC_EPS": "基本每股收益",
            "DEBT_ASSET_RATIO": "资产负债率",
            "CURRENT_RATIO": "流动比率",
            "OPERATE_INCOME_YOY": "营业收入增长率",
            "HOLDER_PROFIT_YOY": "净利润增长率",
        }
        for _, row in df.iterrows():
            year = str(row.get("YEAR", ""))
            if not year:
                continue
            d = {}
            for src_col, dst_name in indicator_map.items():
                if src_col in row and pd.notna(row[src_col]):
                    val = float(row[src_col])
                    # 百分比字段转为小数（适配 A 股分析器习惯）
                    if dst_name in {"毛利率", "资产负债率", "营业收入增长率", "净利润增长率"}:
                        val = val / 100.0
                    d[dst_name] = val
            result[year] = d
        return result
    except Exception as e:
        return {}


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"error": "missing code"}), file=sys.stderr)
        sys.exit(1)

    code = sys.argv[1]
    max_years = 5
    if len(sys.argv) >= 3:
        try:
            max_years = int(sys.argv[2])
            if max_years <= 0:
                max_years = 5
        except ValueError:
            max_years = 5
    result = {
        "symbol": f"HK{code}",
        "years": [],
        "balanceSheet": {},
        "incomeStatement": {},
        "cashFlow": {},
    }

    if ak is None:
        print(json.dumps({"error": "akshare not installed"}), file=sys.stderr)
        sys.exit(1)

    errors = []

    with _suppress_stdout():
        bs_df = fetch_report_sheet(code, "资产负债表")
        is_df = fetch_report_sheet(code, "利润表")
        cf_df = fetch_report_sheet(code, "现金流量表")

    result["balanceSheet"] = add_combined_items(parse_sheet(bs_df))
    result["incomeStatement"] = add_combined_items(parse_sheet(is_df))
    result["cashFlow"] = add_combined_items(parse_sheet(cf_df))

    # 用分析指标补充关键字段（转置为 {item_name: {year: amount}} 结构与 A 股一致）
    with _suppress_stdout():
        indicators = fetch_analysis_indicators(code)
    for year, vals in indicators.items():
        for k, v in vals.items():
            if k not in result["incomeStatement"]:
                result["incomeStatement"][k] = {}
            result["incomeStatement"][k][year] = v

    # 收集所有出现过的年份并排序（降序）
    years_set = set()
    for sheet in [result["balanceSheet"], result["incomeStatement"], result["cashFlow"]]:
        for item_map in sheet.values():
            years_set.update(item_map.keys())
    years = sorted(list(years_set), reverse=True)

    # 限制年份数量
    if max_years > 0 and len(years) > max_years:
        years = years[:max_years]
        # 同步裁剪各表中的超长年份数据
        for sheet in [result["balanceSheet"], result["incomeStatement"], result["cashFlow"]]:
            for item_name in list(sheet.keys()):
                for y in list(sheet[item_name].keys()):
                    if y not in years:
                        del sheet[item_name][y]
    result["years"] = years

    if not result["years"]:
        errors.append("未获取到任何年报数据")

    result["errors"] = errors
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
