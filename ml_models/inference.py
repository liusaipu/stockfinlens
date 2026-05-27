#!/usr/bin/env python3
"""
Unified inference entry for Engine A, B & D.
Go calls this script via stdin/stdout JSON.
"""
import json
import os
import sys
import pickle

# 追踪缺失的依赖包，用于生成友好的错误提示
_MISSING_PACKAGES = []

try:
    import numpy as np
except ImportError:
    np = None
    _MISSING_PACKAGES.append("numpy")

try:
    import onnxruntime as ort
except ImportError:
    ort = None
    _MISSING_PACKAGES.append("onnxruntime")

# Engine D imports
try:
    from sklearn.ensemble import GradientBoostingClassifier
    ENGINE_D_AVAILABLE = True
except ImportError:
    ENGINE_D_AVAILABLE = False
    _MISSING_PACKAGES.append("scikit-learn")


def _make_deps_error():
    """生成依赖缺失的统一错误提示"""
    if not _MISSING_PACKAGES:
        return None
    pkgs = " ".join(sorted(set(_MISSING_PACKAGES)))
    return {
        "error": f"缺少 Python 依赖包: {pkgs}。请在终端运行: pip3 install {pkgs}"
    }


_DEPS_ERROR = _make_deps_error()


SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))

# Load sessions lazily
_session_a = None
_session_b = None
_scaler_b = None
_model_d = None
_feature_stats_d = None


def get_model_d():
    """加载 Engine-D 风险预警模型"""
    global _model_d
    if _model_d is None and ENGINE_D_AVAILABLE:
        model_path = os.path.join(SCRIPT_DIR, "engine_d_risk", "model", "engine_d_model.pkl")
        if os.path.exists(model_path):
            try:
                with open(model_path, "rb") as f:
                    _model_d = pickle.load(f)
            except Exception as e:
                # pickle 加载失败（版本不兼容或文件损坏）时优雅降级
                print(json.dumps({"warning": f"Engine-D 模型加载失败，将使用规则评估: {e}"}), file=sys.stderr)
                _model_d = None
    return _model_d


def get_feature_stats_d():
    """加载 Engine-D 特征统计量（健康/风险均值、方向、importance），用于 per-sample 归因"""
    global _feature_stats_d
    if _feature_stats_d is None:
        stats_path = os.path.join(SCRIPT_DIR, "engine_d_risk", "model", "feature_stats.json")
        if os.path.exists(stats_path):
            try:
                with open(stats_path, "r", encoding="utf-8") as f:
                    _feature_stats_d = json.load(f)
            except Exception:
                _feature_stats_d = {}
        else:
            _feature_stats_d = {}
    return _feature_stats_d


def get_session_a():
    global _session_a
    if _session_a is None:
        path = os.path.join(SCRIPT_DIR, "engine_a_sentiment", "sentiment_price_fusion.onnx")
        _session_a = ort.InferenceSession(path, providers=['CPUExecutionProvider'])
    return _session_a


def get_session_b():
    global _session_b, _scaler_b
    if _session_b is None:
        path = os.path.join(SCRIPT_DIR, "engine_b_financial", "financial_lstm.onnx")
        _session_b = ort.InferenceSession(path, providers=['CPUExecutionProvider'])
        with open(os.path.join(SCRIPT_DIR, "engine_b_financial", "scaler.pkl"), "rb") as f:
            _scaler_b = pickle.load(f)
    return _session_b, _scaler_b


def softmax(x):
    e = np.exp(x - np.max(x, axis=-1, keepdims=True))
    return e / np.sum(e, axis=-1, keepdims=True)


def infer_engine_a(payload):
    """
    payload: {
        "text_seq": [[...], ...],  # [16, 32]
        "price_seq": [[...], ...]  # [16, 24]
    }
    """
    if _DEPS_ERROR is not None:
        return _DEPS_ERROR
    if ort is None:
        return {"error": "缺少 onnxruntime。请在终端运行: pip3 install onnxruntime"}
    text_seq = np.array(payload["text_seq"], dtype=np.float32)
    price_seq = np.array(payload["price_seq"], dtype=np.float32)
    if text_seq.ndim == 2:
        text_seq = np.expand_dims(text_seq, 0)
    if price_seq.ndim == 2:
        price_seq = np.expand_dims(price_seq, 0)

    sess = get_session_a()
    out = sess.run(["movement_logits", "abnormal_prob"], {"text_seq": text_seq, "price_seq": price_seq})
    movement_logits, prob = out
    movement_probs = softmax(movement_logits)[0]
    movement_map = ["down", "flat", "up"]
    direction = movement_map[int(np.argmax(movement_probs))]

    return {
        "direction": direction,
        "direction_probs": {
            "down": round(float(movement_probs[0]), 4),
            "flat": round(float(movement_probs[1]), 4),
            "up": round(float(movement_probs[2]), 4),
        },
        "abnormal_prob": round(float(prob[0][0]), 4),
    }


def infer_engine_b(payload):
    """
    payload: {
        "financial_seq": [[...], ...]  # [8, N_features]
    }
    """
    if _DEPS_ERROR is not None:
        return _DEPS_ERROR
    if ort is None:
        return {"error": "缺少 onnxruntime。请在终端运行: pip3 install onnxruntime"}
    seq = np.array(payload["financial_seq"], dtype=np.float32)
    if seq.ndim == 2:
        seq = np.expand_dims(seq, 0)

    sess, scaler = get_session_b()
    # normalize
    orig_shape = seq.shape
    seq_2d = seq.reshape(-1, seq.shape[-1])
    seq = scaler.transform(seq_2d).reshape(orig_shape).astype(np.float32)

    out = sess.run(["roe_dir", "rev_dir", "mscore_dir", "health_score"], {"financial_seq": seq})
    roe_dir, rev_dir, mscore_dir, health_score = out

    dir_map = ["down", "flat", "up"]
    roe_probs = softmax(roe_dir)[0]
    rev_probs = softmax(rev_dir)[0]
    mscore_probs = softmax(mscore_dir)[0]

    return {
        "roe": {
            "direction": dir_map[int(np.argmax(roe_probs))],
            "confidence": round(float(np.max(roe_probs)), 4),
        },
        "revenue": {
            "direction": dir_map[int(np.argmax(rev_probs))],
            "confidence": round(float(np.max(rev_probs)), 4),
        },
        "mscore": {
            "direction": dir_map[int(np.argmax(mscore_probs))],
            "confidence": round(float(np.max(mscore_probs)), 4),
        },
        "health_score": round(float(health_score[0][0]), 2),
    }


def infer_engine_d(payload):
    """
    Engine D: 风险预警模型推理
    payload: {
        "features": [mscore, zscore, cash_deviation, ar_risk, ...]  # 26维特征
    }
    
    Returns:
    {
        "risk_label": 0/1,  # 0=健康, 1=风险
        "risk_prob": 0.85,  # 风险概率
        "risk_level": "高风险",  # 低风险/中风险/高风险
        "top_factors": ["ascore", "gm_risk", ...]  # 主要风险因子
    }
    """
    if _DEPS_ERROR is not None:
        return _DEPS_ERROR
    model = get_model_d()
    if model is None:
        # 模型未加载，返回基于规则的简单评估
        features = payload.get("features", [])
        if len(features) < 6:
            return {"error": "insufficient features for Engine-D"}
        
        # 简化的规则评估
        ascore = features[5] if len(features) > 5 else 0  # A-Score
        zscore = features[1] if len(features) > 1 else 3  # Z-Score
        mscore = features[0] if len(features) > 0 else -3  # M-Score
        
        risk_score = 0
        if ascore > 60:
            risk_score += 40
        elif ascore > 50:
            risk_score += 20
        if zscore < 1.81:
            risk_score += 30
        elif zscore < 2.99:
            risk_score += 15
        if mscore > -2.22:
            risk_score += 30
        
        risk_prob = min(risk_score / 100.0, 0.99)
        risk_label = 1 if risk_prob > 0.5 else 0
        
        if risk_prob > 0.7:
            risk_level = "高风险"
        elif risk_prob > 0.4:
            risk_level = "中风险"
        else:
            risk_level = "低风险"
        
        return {
            "risk_label": risk_label,
            "risk_prob": round(risk_prob, 4),
            "risk_level": risk_level,
            "top_factors": ["ascore", "zscore", "mscore"] if risk_label == 1 else [],
            "model_loaded": False,
        }
    
    # 使用模型推理
    features = np.array(payload.get("features", []), dtype=np.float32).reshape(1, -1)
    if features.shape[1] != 25:
        return {"error": f"expected 25 features, got {features.shape[1]}"}
    
    risk_proba = model.predict_proba(features)[0]
    risk_label = model.predict(features)[0]
    
    risk_prob = float(risk_proba[1])  # 风险类的概率
    
    if risk_prob > 0.7:
        risk_level = "高风险"
    elif risk_prob > 0.4:
        risk_level = "中风险"
    else:
        risk_level = "低风险"
    
    # Per-sample 风险因子归因
    # 旧逻辑取 model.feature_importances_ 的 top3——对每只股票都是同一个结果，无意义。
    # 新逻辑：(value - healthy_mean) / healthy_std × importance，根据方向打符号，取 top3。
    importances = model.feature_importances_
    feature_names = [
        'mscore', 'zscore', 'cash_deviation', 'ar_risk', 'gm_risk', 'ascore',
        'roe', 'gross_margin', 'revenue_growth', 'debt_ratio', 'ncf_to_profit',
        'goodwill_to_equity', 'inventory_turnover', 'receivable_turnover',
        'pe_ttm', 'pb', 'market_cap', 'turnover_20d', 'volatility_60d', 'max_drawdown_1y',
        'pledge_ratio', 'regulatory_inquiry_count_1y', 'major_shareholder_reduction_1y',
        'auditor_switch_count_2y', 'cfo_change_count_2y'
    ]

    stats = get_feature_stats_d()
    feat_vec = features[0]
    contributions = []
    for i, name in enumerate(feature_names):
        st = stats.get(name)
        if not st:
            continue
        h_mean = st.get('healthy_mean', 0.0)
        h_std = st.get('healthy_std', 1.0) or 1e-6
        direction = st.get('direction', 'higher_is_risk')
        imp = float(importances[i]) if i < len(importances) else 0.0
        z = (float(feat_vec[i]) - h_mean) / h_std
        # higher_is_risk: z 越大越危险；lower_is_risk: z 越小越危险，取负
        signed_z = z if direction == 'higher_is_risk' else -z
        contribution = signed_z * imp
        # 仅保留对风险有正贡献的项
        if contribution > 0:
            label = '偏高' if direction == 'higher_is_risk' else '偏低'
            contributions.append((contribution, name, label, float(feat_vec[i])))

    contributions.sort(key=lambda x: x[0], reverse=True)
    top = contributions[:3]
    top_factors = [f"{name}({label})" for _, name, label, _ in top]

    # 若没有任何正向贡献（极低风险情况），退化为全局重要性 top3 + 标注无显著偏离
    if not top_factors:
        top_indices = np.argsort(importances)[-3:][::-1]
        top_factors = [f"{feature_names[i]}(无显著偏离)" for i in top_indices]

    return {
        "risk_label": int(risk_label),
        "risk_prob": round(risk_prob, 4),
        "risk_level": risk_level,
        "top_factors": top_factors,
        "model_loaded": True,
    }


def main():
    req = json.load(sys.stdin)
    engine = req.get("engine")
    payload = req.get("payload", {})
    try:
        if engine == "A":
            result = infer_engine_a(payload)
        elif engine == "B":
            result = infer_engine_b(payload)
        elif engine == "D":
            result = infer_engine_d(payload)
        else:
            result = {"error": f"unknown engine {engine}"}
        print(json.dumps(result))
    except Exception as e:
        print(json.dumps({"error": str(e)}))


if __name__ == "__main__":
    main()
