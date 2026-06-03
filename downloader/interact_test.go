package downloader

import (
	"context"
	"testing"
	"time"
)

func TestFetchStockInteractQA(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name    string
		market  string
		code    string
		wantErr bool
	}{
		{
			name:    "深市股票",
			market:  "SZ",
			code:    "000001",
			wantErr: false,
		},
		{
			name:    "沪市股票",
			market:  "SH",
			code:    "600000",
			wantErr: false,
		},
		{
			name:    "港股不支持",
			market:  "HK",
			code:    "00700",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qas, err := FetchStockInteractQA(ctx, tt.market, tt.code)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchStockInteractQA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				t.Logf("获取到 %d 条问答数据", len(qas))
				if len(qas) > 0 {
					t.Logf("第一条: Q=%s, A=%s, Date=%s, Source=%s",
						qas[0].Question[:min(20, len(qas[0].Question))],
						qas[0].Answer[:min(30, len(qas[0].Answer))],
						qas[0].Date,
						qas[0].Source)
				}
			}
		})
	}
}

func TestDedupAndFilterQAs(t *testing.T) {
	now := time.Now()
	old := now.AddDate(0, 0, -100)
	recent := now.AddDate(0, 0, -10)

	qas := []InteractQA{
		{Question: "问题1", Answer: "答案1", Date: recent.Format("2006-01-02"), AnswerDate: recent.Format("2006-01-02"), Source: "test"},
		{Question: "问题1", Answer: "答案1重复", Date: recent.Format("2006-01-02"), AnswerDate: recent.Format("2006-01-02"), Source: "test"}, // 重复
		{Question: "问题2", Answer: "答案2", Date: old.Format("2006-01-02"), AnswerDate: old.Format("2006-01-02"), Source: "test"},         // 过期
		{Question: "问题3", Answer: "", Date: recent.Format("2006-01-02"), AnswerDate: recent.Format("2006-01-02"), Source: "test"},      // 无答案
		{Question: "问题4", Answer: "答案4", Date: recent.Format("2006-01-02"), AnswerDate: recent.Format("2006-01-02"), Source: "test"},
	}

	filtered := dedupAndFilterQAs(qas)

	if len(filtered) != 2 {
		t.Errorf("期望过滤后剩余 2 条，实际 %d 条", len(filtered))
	}

	// 验证去重
	questions := make(map[string]bool)
	for _, qa := range filtered {
		if questions[qa.Question] {
			t.Errorf("发现重复问题: %s", qa.Question)
		}
		questions[qa.Question] = true
	}

	// 验证都有答案
	for _, qa := range filtered {
		if qa.Answer == "" {
			t.Errorf("发现空答案: %s", qa.Question)
		}
	}
}
