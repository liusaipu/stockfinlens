#!/usr/bin/env python3
"""
A-Score 第二阶段：非财务风险数据爬虫
输入：stdin JSON {"symbol": "300319.SZ"}
输出：stdout JSON
{
    "pledge_ratio": 1.25,        // 股权质押比例(%), 失败为null
    "inquiry_count_1y": 0,       // 近一年交易所问询函次数, 失败为null
    "reduction_count_1y": 0,     // 近一年大股东减持公告次数, 失败为null
    "error": ""                  // 整体错误信息（如果有）
}
"""
import json
import sys
import datetime
import os
import threading

# 禁用可能的本地代理干扰
for k in ["HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"]:
    os.environ.pop(k, None)

# 全局超时：避免 akshare 网络请求挂起导致 Go 端测试整体超时
CRAWLER_TIMEOUT_SECONDS = 25

def _timeout_exit():
    result = {
        "pledge_ratio": None,
        "inquiry_count_1y": None,
        "reduction_count_1y": None,
        "error": f"爬虫执行超时（>{CRAWLER_TIMEOUT_SECONDS}s）",
    }
    print(json.dumps(result, ensure_ascii=False), flush=True)
    os._exit(1)

_timeout_timer = threading.Timer(CRAWLER_TIMEOUT_SECONDS, _timeout_exit)
_timeout_timer.start()

try:
    import akshare as ak
except ImportError:
    print(json.dumps({"error": "akshare not installed"}), file=sys.stderr)
    sys.exit(1)


def parse_symbol(symbol: str):
    """300319.SZ -> (300319, SZ)"""
    parts = symbol.split(".")
    if len(parts) == 2:
        return parts[0], parts[1].upper()
    return symbol, "SZ"


def fetch_pledge_ratio(code: str):
    """获取股权质押比例（全市场表过滤，约20-30秒）"""
    try:
        df = ak.stock_gpzy_pledge_ratio_em()
        row = df[df["股票代码"] == code]
        if not row.empty:
            return float(row.iloc[0]["质押比例"])
    except Exception as e:
        return None
    return None


def fetch_inquiry_and_reduction_cninfo(code: str, start_date: str, end_date: str):
    """
    通过巨潮资讯网查询近一年公告，按标题关键词统计问询函和减持公告次数。
    失败时返回 (None, None)
    """
    try:
        df = ak.stock_zh_a_disclosure_report_cninfo(
            symbol=code, category="", start_date=start_date, end_date=end_date
        )
        if df.empty:
            return 0, 0
        # 部分版本列名为 "公告标题"，部分为 "标题"
        title_col = None
        for c in ["公告标题", "标题", "announcementTitle"]:
            if c in df.columns:
                title_col = c
                break
        if title_col is None:
            return None, None

        titles = df[title_col].astype(str).tolist()
        inquiry_keywords = ["问询函", "关注函", "监管函", "谈话函"]
        reduction_keywords = ["减持", "减持计划", "减持结果", "减持进展"]

        inquiry_count = sum(
            1 for t in titles if any(k in t for k in inquiry_keywords)
        )
        reduction_count = sum(
            1 for t in titles if any(k in t for k in reduction_keywords)
        )
        return inquiry_count, reduction_count
    except Exception:
        return None, None


def main():
    req = json.load(sys.stdin)
    symbol = req.get("symbol", "")
    code, _ = parse_symbol(symbol)

    end_date = datetime.date.today().strftime("%Y%m%d")
    start_date = (datetime.date.today() - datetime.timedelta(days=365)).strftime("%Y%m%d")

    pledge = fetch_pledge_ratio(code)
    inquiry, reduction = fetch_inquiry_and_reduction_cninfo(code, start_date, end_date)

    result = {
        "pledge_ratio": pledge,
        "inquiry_count_1y": inquiry,
        "reduction_count_1y": reduction,
        "error": "",
    }

    # 如果全部失败，给出提示
    if pledge is None and inquiry is None and reduction is None:
        result["error"] = "所有数据源均不可用"

    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    finally:
        _timeout_timer.cancel()
