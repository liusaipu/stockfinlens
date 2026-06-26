#!/usr/bin/env python3
"""
获取股票历年审计机构信息
数据来源：巨潮资讯网(cninfo.com.cn) 公告查询
输入：{"symbol": "000001.SZ"}
输出：{
  "auditor_name": "xxx",
  "auditor_changed": true/false,
  "history": [...],
  "change_details": [
    {
      "date": "2025-12-11",
      "old_auditor": "永拓",
      "new_auditor": "",
      "reason": "拟变更会计师事务所",
      "is_before_annual_report": true,
      "annual_report_deadline": "2026-04-30",
      "raw_title": "关于拟变更会计师事务所的公告"
    }
  ]
}
"""
import json
import sys
import os
import re
from datetime import datetime, timedelta

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, SCRIPT_DIR)
from cninfo_utils import query_announcements, extract_year_from_title


# 常见会计师事务所名称（用于从标题中提取，按优先级排序：完整名称优先）
KNOWN_AUDITORS = [
    # 完整名称（含组织形式）
    "普华永道中天会计师事务所（特殊普通合伙）",
    "安永华明会计师事务所（特殊普通合伙）",
    "毕马威华振会计师事务所（特殊普通合伙）",
    "德勤华永会计师事务所（特殊普通合伙）",
    "天健会计师事务所（特殊普通合伙）",
    "立信会计师事务所（特殊普通合伙）",
    "大华会计师事务所（特殊普通合伙）",
    "容诚会计师事务所（特殊普通合伙）",
    "天职国际会计师事务所（特殊普通合伙）",
    "信永中和会计师事务所（特殊普通合伙）",
    "中兴华会计师事务所（特殊普通合伙）",
    "中审众环会计师事务所（特殊普通合伙）",
    "大信会计师事务所（特殊普通合伙）",
    "瑞华会计师事务所（特殊普通合伙）",
    "致同会计师事务所（特殊普通合伙）",
    "中汇会计师事务所（特殊普通合伙）",
    "中喜会计师事务所（特殊普通合伙）",
    "中审亚太会计师事务所（特殊普通合伙）",
    "公证天业会计师事务所（特殊普通合伙）",
    "华兴会计师事务所（特殊普通合伙）",
    "永拓会计师事务所（特殊普通合伙）",
    "亚太（集团）会计师事务所（特殊普通合伙）",
    "希格玛会计师事务所（特殊普通合伙）",
    "中准会计师事务所（特殊普通合伙）",
    "上会会计师事务所（特殊普通合伙）",
    "众华会计师事务所（特殊普通合伙）",
    "苏亚金诚会计师事务所（特殊普通合伙）",
    "利安达会计师事务所（特殊普通合伙）",
    # 简称
    "普华永道", "安永", "毕马威", "德勤",
    "天健", "立信", "大华", "容诚", "天职国际", "信永中和",
    "中兴华", "中审众环", "大信", "瑞华", "致同",
    "中汇", "中喜", "中审亚太", "公证天业", "华兴",
    "永拓", "亚太", "希格玛", "中准", "上会", "众华",
    "苏亚金诚", "利安达",
]


def extract_auditor_from_title(title: str) -> str:
    """从标题中提取审计机构名称（返回最完整的匹配）"""
    best_match = ""
    for name in KNOWN_AUDITORS:
        if name in title:
            if len(name) > len(best_match):
                best_match = name
    return best_match


def is_change_announcement(title: str) -> bool:
    """判断是否为审计机构变更类公告"""
    return any(kw in title for kw in ["变更", "更换", "改聘", "解聘", "不再续聘", "终止合作"])


# 政策合规更换关键词（国企8年强制轮换等）
POLICY_COMPLIANCE_KEYWORDS = [
    "轮换期届满", "聘任期限届满", "服务年限届满", "强制轮换",
    "聘期届满", "合同期限届满", "审计年限届满", "连续服务年限",
    "达到轮换年限", "轮换期限", "服务期满",
]

# 被动更换关键词（原事务所被处罚/禁入等，非公司自身问题）
PASSIVE_CHANGE_KEYWORDS = [
    "被证监会处罚", "被暂停", "被禁入", "监管决定", "执业受限",
    "独立性受限", "被监管机构", "行政处罚", "市场禁入",
    "资格暂停", "签字会计师", "恒大", "普华永道",
]

# 正常轮换关键词（看似异常实则正常）
NORMAL_ROTATION_KEYWORDS = [
    "招标", "选聘", "竞聘", "评标",
    "不存在分歧", "无分歧", "双方协商",
    "连续服务", "服务期满", "连续审计",
    "保独立性", "保客观性", "客观性",
    "年限届满", "轮换期",
]

# 异常更换关键词（需警惕）
ABNORMAL_KEYWORDS = [
    "无法达成一致", "审计范围受限", "审计意见分歧",
    "辞任", "辞聘", "主动辞任", "被解聘",
]

# 已知曾被证监会行政处罚或暂停证券业务的事务所名单
# 格式：事务所简称 -> [(处罚开始日, 处罚结束日/预计结束日), ...]
# 变更日期落在处罚期内或结束后 90 天内，视为公司被动更换，不列为风险
PENALIZED_AUDITORS = {
    "天职国际": [("2024-08-01", "2025-02-01")],
    "普华永道": [("2024-09-01", "2025-03-01")],
    "大华": [("2024-05-01", "2024-11-01")],
}


def infer_change_reason(title: str) -> str:
    """从标题推断变更原因"""
    if "不再续聘" in title or "终止合作" in title:
        return "不再续聘原审计机构"
    if "解聘" in title:
        return "解聘原审计机构"
    if "改聘" in title:
        return "改聘审计机构"
    if "拟变更" in title or "拟改聘" in title:
        return "拟变更审计机构"
    if "变更" in title:
        return "变更审计机构"
    if "更换" in title:
        return "更换审计机构"
    return "审计机构变更"


def is_policy_compliance_change(title: str) -> bool:
    """判断是否为政策合规更换（如国企8年强制轮换）"""
    return any(kw in title for kw in POLICY_COMPLIANCE_KEYWORDS)


def is_passive_change(title: str) -> bool:
    """判断是否为被动更换（原事务所被处罚/禁入等，非公司自身问题）"""
    return any(kw in title for kw in PASSIVE_CHANGE_KEYWORDS)


def is_normal_rotation_change(title: str) -> bool:
    """判断是否为正常轮换更换（看似异常实则正常）"""
    return any(kw in title for kw in NORMAL_ROTATION_KEYWORDS)


def is_abnormal_change(title: str) -> bool:
    """判断是否为异常更换（需警惕）"""
    return any(kw in title for kw in ABNORMAL_KEYWORDS)


def is_before_annual_report(date_str: str) -> tuple:
    """判断变更日期是否在年报披露期间（1月1日-4月30日）
    返回: (是否年报前, 对应年报截止日)"""
    try:
        dt = datetime.strptime(date_str, "%Y-%m-%d")
        month = dt.month
        if 1 <= month <= 4:
            # 年报截止日为同年4月30日
            return True, f"{dt.year}-04-30"
        else:
            # 变更日期在5月及之后，对应次年年报
            return False, f"{dt.year + 1}-04-30"
    except Exception:
        return False, ""


def is_penalized_passive_change(old_auditor: str, change_date_str: str) -> bool:
    """根据已知的受罚事务所名单，判断是否为被动更换
    变更日期落在处罚期内或结束后 90 天内，视为被动更换
    """
    if not old_auditor or not change_date_str:
        return False
    try:
        change_dt = datetime.strptime(change_date_str, "%Y-%m-%d")
    except Exception:
        return False

    for key, periods in PENALIZED_AUDITORS.items():
        # 支持简称和全名的互相包含匹配
        if key not in old_auditor and old_auditor not in key:
            continue
        for start_str, end_str in periods:
            try:
                start_dt = datetime.strptime(start_str, "%Y-%m-%d")
                end_dt = datetime.strptime(end_str, "%Y-%m-%d")
                # 处罚期内 + 结束后 90 天缓冲期
                if start_dt <= change_dt <= end_dt + timedelta(days=90):
                    return True
            except Exception:
                continue
    return False


def build_change_details(announcements: list) -> list:
    """从公告列表中构建结构化变更详情"""
    change_details = []
    
    # 按日期排序
    sorted_anns = sorted(announcements, key=lambda x: x.get("announcementDate", ""))
    
    for i, a in enumerate(sorted_anns):
        title = a.get("announcementTitle", "")
        date = a.get("announcementDate", "")
        
        if not is_change_announcement(title):
            continue
        
        # 推断旧事务所：查找此前最近的一个含审计机构名称的公告
        old_auditor = ""
        for j in range(i - 1, -1, -1):
            prev_title = sorted_anns[j].get("announcementTitle", "")
            prev_auditor = extract_auditor_from_title(prev_title)
            if prev_auditor:
                old_auditor = prev_auditor
                break
        
        # 推断新事务所：查找此后最近的一个含审计机构名称的公告
        new_auditor = ""
        for j in range(i + 1, len(sorted_anns)):
            next_title = sorted_anns[j].get("announcementTitle", "")
            next_auditor = extract_auditor_from_title(next_title)
            if next_auditor and next_auditor != old_auditor:
                new_auditor = next_auditor
                break
        
        is_before, deadline = is_before_annual_report(date)
        
        is_abnormal_flag = is_abnormal_change(title)
        is_normal_flag = is_normal_rotation_change(title)
        # 如果一个标题同时匹配正常和异常关键词，优先异常（异常信号更严重）
        # 但如果只有正常关键词（如"招标"、"选聘"），则判定为正常轮换
        if is_normal_flag and is_abnormal_flag:
            is_normal_flag = False

        # 标题关键词判断 + 受罚事务所名单双重判定被动更换
        is_passive_flag = is_passive_change(title)
        if not is_passive_flag and old_auditor:
            is_passive_flag = is_penalized_passive_change(old_auditor, date)

        change_details.append({
            "date": date,
            "old_auditor": old_auditor,
            "new_auditor": new_auditor,
            "reason": infer_change_reason(title),
            "is_before_annual_report": is_before,
            "annual_report_deadline": deadline,
            "raw_title": title,
            "is_policy_compliance": is_policy_compliance_change(title),
            "is_abnormal": is_abnormal_flag,
            "is_passive_change": is_passive_flag,
            "is_normal_rotation": is_normal_flag,
        })
    
    return change_details


def fetch_auditor_history(symbol: str):
    """获取股票历年审计机构信息"""
    try:
        # 查询近3年的会计师事务所相关公告
        start = (datetime.now() - timedelta(days=365 * 3)).strftime("%Y-%m-%d")
        announcements = query_announcements(
            symbol,
            search_key="会计师事务所",
            start_date=start,
            page_size=50,
        )

        if announcements and "error" in announcements[0]:
            return {
                "auditor_name": "",
                "auditor_changed": False,
                "history": [],
                "change_details": [],
                "error": announcements[0]["error"],
            }

        # 按日期排序（用于历史推断）
        sorted_anns = sorted(announcements, key=lambda x: x.get("announcementDate", ""))
        
        history_records = []
        has_change = False
        current_auditor = ""
        
        # 查找当前审计机构（最新一条含审计机构名称的公告中的机构）
        for a in reversed(sorted_anns):
            title = a.get("announcementTitle", "")
            # 跳过变更公告本身
            if is_change_announcement(title):
                continue
            auditor = extract_auditor_from_title(title)
            if auditor:
                current_auditor = auditor
                break
        
        for a in sorted_anns:
            title = a.get("announcementTitle", "")
            date = a.get("announcementDate", "")
            is_change = is_change_announcement(title)
            if is_change:
                has_change = True
            # 尝试提取审计机构名称
            auditor = extract_auditor_from_title(title)
            year = extract_year_from_title(title) or date[:4]
            change_flag = " [变更]" if is_change else ""
            auditor_str = f" ({auditor})" if auditor else ""
            history_records.append(f"[{year}] [{date}]{change_flag}{auditor_str} {title}")

        # 构建结构化变更详情
        change_details = build_change_details(sorted_anns)

        return {
            "auditor_name": current_auditor,
            "auditor_changed": has_change,
            "history": history_records[:10],
            "change_details": change_details,
        }

    except Exception as e:
        return {
            "auditor_name": "",
            "auditor_changed": False,
            "history": [],
            "change_details": [],
            "error": str(e),
        }


# 审计意见类型关键词映射
OPINION_KEYWORDS = [
    ("否定意见", "否定意见"),
    ("无法表示意见", "无法表示意见"),
    ("保留意见", "保留意见"),
    ("强调事项", "带强调事项段的无保留意见"),
    ("关键审计事项", "标准无保留意见"),  # 关键审计事项属于标准意见的一部分
]


def extract_opinion_from_title(title: str) -> str:
    """从公告标题中提取审计意见类型"""
    for kw, opinion in OPINION_KEYWORDS:
        if kw in title:
            return opinion
    # 如果标题只含"审计报告"而无上述关键词，推测为标准无保留
    if "审计报告" in title:
        return "标准无保留意见（推测）"
    return ""


def fetch_audit_opinions(symbol: str):
    """获取股票历年审计意见（基于审计报告公告标题推断）"""
    try:
        # 查询近5年的审计报告公告
        start = (datetime.now() - timedelta(days=365 * 5)).strftime("%Y-%m-%d")
        announcements = query_announcements(
            symbol,
            search_key="审计报告",
            start_date=start,
            page_size=50,
        )

        if announcements and "error" in announcements[0]:
            return []

        opinions = []
        seen_years = set()
        # 按日期降序，取每年最新的审计报告
        sorted_anns = sorted(announcements, key=lambda x: x.get("announcementDate", ""), reverse=True)
        for a in sorted_anns:
            title = a.get("announcementTitle", "")
            date = a.get("announcementDate", "")
            # 只处理年度审计报告（排除半年报、季报审计）
            if "半年" in title or "季度" in title or "季度" in title or "内部控制" in title:
                continue
            year = extract_year_from_title(title) or date[:4]
            if not year or year in seen_years:
                continue
            opinion = extract_opinion_from_title(title)
            if not opinion:
                continue
            seen_years.add(year)
            # 提取审计机构名称
            auditor = extract_auditor_from_title(title)
            opinions.append({
                "year": year,
                "opinion": opinion,
                "auditor": auditor,
                "is_standard": "标准无保留" in opinion and "推测" not in opinion,
                "needs_review": "推测" in opinion,
                "announcement_date": date,
                "raw_title": title,
            })
        return opinions
    except Exception as e:
        return [{"error": str(e)}]


def main():
    req = json.load(sys.stdin)
    symbol = req.get("symbol", "")
    result = fetch_auditor_history(symbol)
    # 同时获取审计意见
    result["audit_opinions"] = fetch_audit_opinions(symbol)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()
