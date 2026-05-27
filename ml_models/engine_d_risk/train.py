#!/usr/bin/env python3
"""
Engine-D: GradientBoosting 风险预警模型训练

输入特征 (26维):
- 财务指标 (14维): mscore, zscore, cash_deviation, ar_risk, gm_risk, ascore, 
  roe, gross_margin, revenue_growth, debt_ratio, ncf_to_profit, 
  goodwill_to_equity, inventory_turnover, receivable_turnover
- 市场指标 (6维): pe_ttm, pb, market_cap, turnover_20d, volatility_60d, max_drawdown_1y
- 非财务指标 (6维): pledge_ratio, regulatory_inquiry_count_1y, 
  major_shareholder_reduction_1y, auditor_switch_count_2y, cfo_change_count_2y

输出标签 (3分类):
- 0: 健康
- 1: 财务造假/重大违规
- 2: 退市风险

用法:
  python train.py --data-dir ../../data/fraud_dataset --output-dir ./model
"""

import argparse
import json
import os
import pickle
import sys
from datetime import datetime
from pathlib import Path

import numpy as np
import pandas as pd
from sklearn.model_selection import train_test_split
from sklearn.metrics import classification_report
from sklearn.ensemble import GradientBoostingClassifier
import warnings
warnings.filterwarnings('ignore')


def load_json(path):
    with open(path, 'r', encoding='utf-8') as f:
        return json.load(f)


def save_json(path, data):
    os.makedirs(os.path.dirname(path) if os.path.dirname(path) else '.', exist_ok=True)
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def generate_synthetic_features(samples, seed=42):
    """
    生成合成特征数据用于训练演示
    实际场景应该从真实的财务数据中提取
    
    特征规则：
    - 正样本（风险）：M-Score 偏高、现金流偏离度大、应收异常等
    - 负样本（健康）：各项指标正常
    """
    np.random.seed(seed)
    
    features_list = []
    for sample in samples:
        label = sample.get('label', 0)
        
        # 基础特征（根据标签调整分布）
        if label == 1:  # 风险（退市/违规）
            # M-Score 偏高 (> -2.22 表示风险)
            mscore = np.random.normal(-1.8, 0.9)
            # Z-Score 偏低 (< 1.81 表示破产风险)
            zscore = np.random.normal(1.2, 0.8)
            # 现金流偏离度大
            cash_dev = np.random.normal(0.28, 0.14)
            # 应收账款异常
            ar_risk = np.random.normal(0.5, 0.2)
            # 毛利率风险（单边：YoY 跌幅小数，约 0-15pp）
            gm_risk = max(0.0, np.random.normal(0.08, 0.06))
            # A-Score 偏高
            ascore = np.random.normal(70, 15)
            # ROE 可能虚高或偏低
            roe = np.random.choice([
                np.random.normal(0.25, 0.1),  # 虚高
                np.random.normal(0.02, 0.02),  # 偏低
                np.random.normal(-0.05, 0.08),  # 亏损
            ])
            # 负债率偏高
            debt_ratio = np.random.normal(0.75, 0.15)
            # 净利润现金含量低
            ncf_to_profit = np.random.normal(0.4, 0.35)
            
        else:  # 健康
            mscore = np.random.normal(-2.8, 0.4)  # 正常 <-2.22
            zscore = np.random.normal(3.5, 1.0)   # 健康 > 2.99
            cash_dev = np.random.normal(0.05, 0.08)
            ar_risk = np.random.normal(0.15, 0.1)
            # 毛利率风险（单边）：健康公司毛利稳定/上升，gm_risk ~ 0
            gm_risk = max(0.0, np.random.normal(0.0, 0.02))
            ascore = np.random.normal(35, 12)
            roe = np.random.normal(0.12, 0.06)
            debt_ratio = np.random.normal(0.45, 0.15)
            ncf_to_profit = np.random.normal(1.1, 0.3)
        
        # 其他财务指标
        gross_margin = np.random.normal(0.35, 0.15) if label == 0 else np.random.normal(0.25, 0.2)
        revenue_growth = np.random.normal(0.15, 0.1) if label == 0 else np.random.normal(-0.05, 0.2)
        goodwill_to_equity = np.random.normal(0.1, 0.08) if label == 0 else np.random.normal(0.25, 0.15)
        inventory_turnover = np.random.normal(6, 3) if label == 0 else np.random.normal(3, 2)
        receivable_turnover = np.random.normal(8, 4) if label == 0 else np.random.normal(4, 3)
        
        # 市场指标
        pe_ttm = np.random.normal(25, 15) if label == 0 else np.random.normal(60, 40)
        pb = np.random.normal(2.5, 1.5) if label == 0 else np.random.normal(4, 3)
        market_cap = np.random.lognormal(5, 1)  # 亿元
        turnover_20d = np.random.normal(0.03, 0.02)
        volatility_60d = np.random.normal(0.3, 0.1) if label == 0 else np.random.normal(0.5, 0.2)
        max_drawdown_1y = np.random.normal(-0.15, 0.08) if label == 0 else np.random.normal(-0.35, 0.15)
        
        # 非财务指标
        pledge_ratio = np.random.normal(0.15, 0.1) if label == 0 else np.random.normal(0.4, 0.2)
        regulatory_inquiry_count_1y = np.random.poisson(0.2) if label == 0 else np.random.poisson(2)
        major_shareholder_reduction_1y = np.random.poisson(0.5) if label == 0 else np.random.poisson(3)
        auditor_switch_count_2y = np.random.poisson(0.1) if label == 0 else np.random.poisson(1)
        cfo_change_count_2y = np.random.poisson(0.2) if label == 0 else np.random.poisson(1.5)
        
        features = {
            'code': sample.get('code', ''),
            'name': sample.get('name', ''),
            'label': label,
            # 财务指标
            'mscore': mscore,
            'zscore': zscore,
            'cash_deviation': cash_dev,
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
            # 市场指标
            'pe_ttm': pe_ttm,
            'pb': pb,
            'market_cap': market_cap,
            'turnover_20d': turnover_20d,
            'volatility_60d': volatility_60d,
            'max_drawdown_1y': max_drawdown_1y,
            # 非财务指标
            'pledge_ratio': pledge_ratio,
            'regulatory_inquiry_count_1y': regulatory_inquiry_count_1y,
            'major_shareholder_reduction_1y': major_shareholder_reduction_1y,
            'auditor_switch_count_2y': auditor_switch_count_2y,
            'cfo_change_count_2y': cfo_change_count_2y,
        }
        features_list.append(features)
    
    return pd.DataFrame(features_list)


def train_model(data_dir: str, output_dir: str):
    """训练 GradientBoosting 模型"""
    
    print("=" * 60)
    print("Engine-D GradientBoosting 风险预警模型训练")
    print("=" * 60)
    
    # 加载原始案例（更详细的标签信息）
    raw_cases_path = os.path.join(data_dir, 'raw_cases.json')
    raw_cases = load_json(raw_cases_path)
    
    # 构建带标签的样本（二分类：0=健康, 1=风险）
    positive_samples = []
    
    # 退市股票 -> 标签 1
    for s in raw_cases.get('delisted_stocks', []):
        s['label'] = 1
        positive_samples.append(s)
    
    # 财务造假 -> 标签 1
    for s in raw_cases.get('fraud_stocks', []):
        s['label'] = 1
        positive_samples.append(s)
    
    # 去重
    seen = set()
    unique_positive = []
    for s in positive_samples:
        code = s.get('code', '')
        if code and code not in seen:
            seen.add(code)
            unique_positive.append(s)
    positive_samples = unique_positive
    
    # 加载负样本
    negative_path = os.path.join(data_dir, 'negative', 'samples.json')
    negative_samples = load_json(negative_path)
    
    # 负样本标签为 0
    for s in negative_samples:
        s['label'] = 0
    
    all_samples = positive_samples + negative_samples
    print(f"\n加载样本: 正样本 {len(positive_samples)}, 负样本 {len(negative_samples)}")
    print(f"标签分布: 健康={sum(1 for s in all_samples if s['label']==0)}, "
          f"风险={sum(1 for s in all_samples if s['label']==1)}")
    
    # 生成特征
    print("\n生成合成特征数据...")
    df = generate_synthetic_features(all_samples)
    
    # 特征列
    feature_cols = [
        'mscore', 'zscore', 'cash_deviation', 'ar_risk', 'gm_risk', 'ascore',
        'roe', 'gross_margin', 'revenue_growth', 'debt_ratio', 'ncf_to_profit',
        'goodwill_to_equity', 'inventory_turnover', 'receivable_turnover',
        'pe_ttm', 'pb', 'market_cap', 'turnover_20d', 'volatility_60d', 'max_drawdown_1y',
        'pledge_ratio', 'regulatory_inquiry_count_1y', 'major_shareholder_reduction_1y',
        'auditor_switch_count_2y', 'cfo_change_count_2y'
    ]
    
    X = df[feature_cols].values
    y = df['label'].values
    
    # 数据分割
    X_train, X_test, y_train, y_test = train_test_split(
        X, y, test_size=0.2, random_state=42, stratify=y
    )
    
    print(f"\n训练集: {len(X_train)}, 测试集: {len(X_test)}")
    
    # 训练 GradientBoosting（二分类）
    print("\n训练 GradientBoosting 模型...")
    
    model = GradientBoostingClassifier(
        n_estimators=100,
        max_depth=4,
        learning_rate=0.1,
        subsample=0.8,
        random_state=42,
    )
    
    model.fit(X_train, y_train)
    
    # 评估
    print("\n模型评估...")
    y_pred = model.predict(X_test)
    y_pred_proba = model.predict_proba(X_test)
    
    print("\n分类报告:")
    print(classification_report(y_test, y_pred, 
                               target_names=['健康', '风险']))
    
    # 特征重要性
    print("\n特征重要性 (Top 15):")
    importance = model.feature_importances_
    feature_importance = list(zip(feature_cols, importance))
    feature_importance.sort(key=lambda x: x[1], reverse=True)
    for feat, imp in feature_importance[:15]:
        print(f"  {feat}: {imp:.4f}")
    
    # 保存模型
    os.makedirs(output_dir, exist_ok=True)
    
    model_path = os.path.join(output_dir, 'engine_d_model.pkl')
    with open(model_path, 'wb') as f:
        pickle.dump(model, f)
    print(f"\n✓ 模型已保存: {model_path}")
    
    # 保存特征列表
    feature_config = {
        'features': feature_cols,
        'feature_importance': [{k: float(v)} for k, v in feature_importance],
        'model_params': {
            'n_estimators': model.n_estimators,
            'max_depth': model.max_depth,
            'learning_rate': model.learning_rate,
        },
        'train_info': {
            'train_time': datetime.now().isoformat(),
            'n_samples': len(all_samples),
            'n_features': len(feature_cols),
        }
    }
    config_path = os.path.join(output_dir, 'config.json')
    save_json(config_path, feature_config)
    print(f"✓ 配置已保存: {config_path}")

    # 保存每个特征在「健康类」上的均值/标准差，供推理时做 per-sample 风险归因
    # 用法：z = (value - healthy_mean) / healthy_std，z 越大表示该特征越偏离健康水平
    healthy_df = df[df['label'] == 0]
    risk_df = df[df['label'] == 1]
    feature_stats = {}
    for col in feature_cols:
        h_mean = float(healthy_df[col].mean())
        h_std = float(healthy_df[col].std()) or 1e-6
        r_mean = float(risk_df[col].mean())
        # 方向：风险均值在健康均值之上 → "偏高"加重风险；反之 "偏低"加重风险
        direction = 'higher_is_risk' if r_mean > h_mean else 'lower_is_risk'
        feature_stats[col] = {
            'healthy_mean': h_mean,
            'healthy_std': h_std,
            'risk_mean': r_mean,
            'direction': direction,
            'importance': float(dict(feature_importance)[col]),
        }
    stats_path = os.path.join(output_dir, 'feature_stats.json')
    save_json(stats_path, feature_stats)
    print(f"✓ 特征统计已保存: {stats_path}")
    
    # 保存测试集预测示例
    sample_predictions = []
    for i in range(min(10, len(X_test))):
        sample_predictions.append({
            'code': df.iloc[i]['code'],
            'name': df.iloc[i]['name'],
            'true_label': int(y_test[i]),
            'pred_label': int(y_pred[i]),
            'probs': {
                '健康': round(float(y_pred_proba[i][0]), 4),
                '风险': round(float(y_pred_proba[i][1]), 4),
            }
        })
    
    sample_path = os.path.join(output_dir, 'sample_predictions.json')
    save_json(sample_path, sample_predictions)
    print(f"✓ 预测示例已保存: {sample_path}")
    
    return model, feature_cols


def main():
    parser = argparse.ArgumentParser(description='Train Engine-D GradientBoosting Risk Model')
    parser.add_argument('--data-dir', default='../../data/fraud_dataset',
                       help='Path to fraud dataset directory')
    parser.add_argument('--output-dir', default='./model',
                       help='Output directory for model files')
    
    args = parser.parse_args()
    
    model, feature_cols = train_model(args.data_dir, args.output_dir)
    
    print("\n" + "=" * 60)
    print("训练完成！")
    print("=" * 60)


if __name__ == '__main__':
    main()
