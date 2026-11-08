# -*- coding: utf-8 -*-
"""刷新内置股票代码库（A 股沪/深/北 + 港股）。

数据源：
- A 股：akshare stock_info_a_code_name（交易所官方页面，非东财）
- 港股名单：港交所 ListOfSecurities.xlsx（Category=Equity，剔除 REIT）
- 港股中文名：腾讯行情接口 qt.gtimg.cn（GBK，批量查询；失败时回退港交所英文名）

stdout 只输出一行 JSON：{"success": true, "stocks": [...]}
任何进度/调试信息一律走 stderr，禁止破坏 stdout JSON 协议。
"""
import io
import json
import re
import sys
import time

import pandas as pd
import requests

HKEX_XLSX_URL = (
    "https://www.hkex.com.hk/eng/services/trading/securities/"
    "securitieslists/ListOfSecurities.xlsx"
)
TENCENT_QUOTE_URL = "https://qt.gtimg.cn/q="


def log(msg):
    print(msg, file=sys.stderr, flush=True)


def fetch_a_shares():
    """拉取 A 股（沪/深/北）最新名单，返回 {(market, code): name}"""
    import akshare as ak

    last_err = None
    for attempt in range(3):
        try:
            df = ak.stock_info_a_code_name()
            entries = {}
            for code, name in zip(df["code"], df["name"]):
                code = str(code).strip().zfill(6)
                name = strip_listing_marker(str(name).strip())
                if code[0] == "6":
                    market = "SH"
                elif code[0] in "03":
                    market = "SZ"
                else:
                    market = "BJ"
                entries[(market, code)] = name
            if len(entries) < 4000:
                raise RuntimeError(f"A 股名单数量异常: {len(entries)}")
            return entries
        except Exception as e:  # noqa: BLE001
            last_err = e
            log(f"A 股名单获取失败（第 {attempt + 1} 次）: {e}")
            time.sleep(2)
    raise RuntimeError(f"A 股名单获取失败: {last_err}")


def strip_listing_marker(name):
    """剔除新股上市首日(N)/次日至第五日(C/XC)的临时简称标记"""
    return re.sub(r"^(XC|N|C)(?=[一-鿿])", "", name)


def fetch_hk_universe():
    """从港交所官方名单获取港股代码池，返回 {code5: english_name}"""
    last_err = None
    for attempt in range(3):
        try:
            resp = requests.get(
                HKEX_XLSX_URL,
                timeout=60,
                headers={"User-Agent": "Mozilla/5.0"},
            )
            resp.raise_for_status()
            raw = resp.content
            df = pd.read_excel(io.BytesIO(raw), sheet_name="ListOfSecurities", header=2)
            df.columns = [str(c).strip() for c in df.columns]
            eq = df[df["Category"] == "Equity"]
            # 与内置库口径一致：不含 REIT（Investment Trust）
            eq = eq[~eq["Sub-Category"].astype(str).str.contains("Investment Trust")]
            result = {}
            for code, name in zip(eq["Stock Code"], eq["Name of Securities"]):
                result[str(code).strip().zfill(5)] = str(name).strip()
            if len(result) < 2000:
                raise RuntimeError(f"港股名单数量异常: {len(result)}")
            return result
        except Exception as e:  # noqa: BLE001
            last_err = e
            log(f"港股名单获取失败（第 {attempt + 1} 次）: {e}")
            time.sleep(2)
    raise RuntimeError(f"港股名单获取失败: {last_err}")


def fetch_hk_chinese_names(codes):
    """通过腾讯行情接口批量获取港股中文简称，返回 {code5: cn_name}"""
    names = {}
    for i in range(0, len(codes), 50):
        batch = codes[i : i + 50]
        url = TENCENT_QUOTE_URL + ",".join("hk" + c for c in batch)
        last_err = None
        for attempt in range(2):
            try:
                resp = requests.get(url, timeout=20, headers={"User-Agent": "Mozilla/5.0"})
                resp.raise_for_status()
                text = resp.content.decode("gbk", errors="replace")
                for part in text.strip().split(";"):
                    part = part.strip()
                    if not part.startswith("v_hk"):
                        continue
                    var, _, val = part.partition("=")
                    fields = val.strip('"').split("~")
                    if len(fields) > 1 and fields[1]:
                        names[var[4:]] = fields[1].strip()
                last_err = None
                break
            except Exception as e:  # noqa: BLE001
                last_err = e
                time.sleep(1)
        if last_err:
            log(f"港股中文名获取部分失败（{batch[0]} 起）: {last_err}")
        time.sleep(0.3)
    return names


def main():
    a_entries = fetch_a_shares()
    log(f"A 股（含北交所）: {len(a_entries)} 只")

    hk_universe = fetch_hk_universe()
    log(f"港交所 Equity（剔除 REIT）: {len(hk_universe)} 只")

    hk_names = fetch_hk_chinese_names(sorted(hk_universe))
    log(f"港股中文名获取: {len(hk_names)}/{len(hk_universe)}")

    stocks = []
    for (market, code), name in sorted(a_entries.items(), key=lambda x: (x[0][0], x[0][1])):
        stocks.append({"code": f"{code}.{market}", "name": name, "market": market})
    for code5 in sorted(hk_universe):
        name = hk_names.get(code5, hk_universe[code5])
        stocks.append({"code": f"{code5}.HK", "name": name, "market": "HK"})

    # 排序口径与内置 data/stocks.json 一致：BJ -> HK -> SH -> SZ
    market_order = {"BJ": 0, "HK": 1, "SH": 2, "SZ": 3}
    stocks.sort(key=lambda d: (market_order[d["market"]], d["code"]))

    print(json.dumps({"success": True, "stocks": stocks}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as e:  # noqa: BLE001
        log(f"刷新失败: {e}")
        print(json.dumps({"success": False, "error": str(e)}, ensure_ascii=False))
        sys.exit(0)
