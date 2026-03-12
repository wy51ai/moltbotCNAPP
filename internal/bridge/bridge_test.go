package bridge

import (
	"testing"

	"github.com/wy51ai/moltbotCNAPP/internal/feishu"
)

func TestShouldRespondInGroup(t *testing.T) {
	testCases := []struct {
		name     string
		text     string
		mentions []feishu.Mention
		want     bool
	}{
		{
			name: "responds when mentioned",
			text: "hello there",
			mentions: []feishu.Mention{
				{ID: "user-1"},
			},
			want: true,
		},
		{
			name: "responds to trailing question mark",
			text: "can you help?",
			want: true,
		},
		{
			name: "responds to english question word",
			text: "How does this work",
			want: true,
		},
		{
			name: "responds to chinese action verb",
			text: "麻烦帮我看一下",
			want: true,
		},
		{
			name: "responds to bot name prefix",
			text: "bot: status",
			want: true,
		},
		{
			name: "responds to exact bot name",
			text: "助手",
			want: true,
		},
		{
			name: "ignores unrelated group chat",
			text: "今天中午吃什么",
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRespondInGroup(tc.text, tc.mentions); got != tc.want {
				t.Fatalf("shouldRespondInGroup(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestRemoveMentions(t *testing.T) {
	got := removeMentions("@_user_123 请帮我 @_user_456 看看")
	want := "请帮我 看看"

	if got != want {
		t.Fatalf("removeMentions() = %q, want %q", got, want)
	}
}
