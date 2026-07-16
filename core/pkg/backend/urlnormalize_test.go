package backend

import (
	"reflect"
	"testing"
)

func TestCandidateOpenAIAPIRoots(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{
			"https://api.ppio.com/openai",
			[]string{"https://api.ppio.com/openai/v1", "https://api.ppio.com/openai"},
		},
		{
			"https://coding.dashscope.aliyuncs.com/v1",
			[]string{"https://coding.dashscope.aliyuncs.com/v1"},
		},
		{
			"https://api.openai.com",
			[]string{"https://api.openai.com/v1", "https://api.openai.com"},
		},
	}
	for _, tc := range cases {
		got := CandidateOpenAIAPIRoots(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("CandidateOpenAIAPIRoots(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}
}
