package processor

import "fmt"

// QAToCacheEntries 将拆分结果转换为缓存条目
// 这个辅助函数用于将拆分后的问答对转换为可以存储到缓存的格式
func QAToCacheEntries(result *QASplitResult, model string) []map[string]interface{} {
	entries := make([]map[string]interface{}, 0, len(result.QAPairs))
	for i, pair := range result.QAPairs {
		entries = append(entries, map[string]interface{}{
			"question": pair.Question,
			"answer":   pair.Answer,
			"index":    i,
			"split":    result.Split,
			"model":    model,
		})
	}
	return entries
}

// FlattenQA 从拆分结果中提取所有问题和答案
func FlattenQA(result *QASplitResult) ([]string, []string) {
	questions := make([]string, len(result.QAPairs))
	answers := make([]string, len(result.QAPairs))
	for i, pair := range result.QAPairs {
		questions[i] = pair.Question
		answers[i] = pair.Answer
	}
	return questions, answers
}

// FormatQAPair 格式化问答对为字符串
func FormatQAPair(pair QAPair) string {
	return fmt.Sprintf("Q: %s\nA: %s", pair.Question, pair.Answer)
}
