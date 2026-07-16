package cache

import "testing"

func TestExtractCleanQuestion_RemovesSenderMetadataAndTimestamp(t *testing.T) {
	input := "Sender (untrusted metadata):\n```json\n{\n  \"label\": \"openclaw-control-ui\",\n  \"id\": \"openclaw-control-ui\"\n}\n```\n\n[Sat 2026-03-21 16:35 UTC] 你大爷的"

	got := extractCleanQuestion(input)
	want := "你大爷的"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}

func TestExtractCleanQuestion_StripsTimestampPrefixOnly(t *testing.T) {
	input := "[Sat 2026-03-21 16:35 UTC] 你使用的什么模型"
	got := extractCleanQuestion(input)
	want := "你使用的什么模型"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}

func TestExtractCleanQuestion_LeavesNormalQuestionUntouched(t *testing.T) {
	input := "请介绍一下你自己"
	got := extractCleanQuestion(input)
	want := input
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, input)
	}
}

func TestExtractCleanQuestion_RemovesGMTPrefix(t *testing.T) {
	input := "19 GMT+8] 帮我看看我电脑上，运行了那些容器，挂载了那些目录"
	got := extractCleanQuestion(input)
	want := "帮我看看我电脑上，运行了那些容器，挂载了那些目录"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}

func TestExtractCleanQuestion_RemovesGMTWithOffset(t *testing.T) {
	input := "[Sat 2026-03-21 16:35 GMT+8] 测试问题"
	got := extractCleanQuestion(input)
	want := "测试问题"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}

func TestExtractCleanQuestion_RemovesGMTWithMinutes(t *testing.T) {
	input := "[Sat 2026-03-21 16:35 GMT+8:00] 测试问题"
	got := extractCleanQuestion(input)
	want := "测试问题"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}

func TestExtractCleanQuestion_RemovesGMTMinusOffset(t *testing.T) {
	input := "[Sat 2026-03-21 16:35 GMT-5] 测试问题"
	got := extractCleanQuestion(input)
	want := "测试问题"
	if got != want {
		t.Fatalf("extractCleanQuestion() = %q, want %q", got, want)
	}
}
