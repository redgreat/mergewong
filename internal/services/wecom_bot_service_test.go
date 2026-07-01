package services

import "testing"

func TestNormalizeWecomRobotID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		bad   bool
	}{
		{name: "robot id", input: "abc-123", want: "abc-123"},
		{name: "webhook", input: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc-123", want: "abc-123"},
		{name: "missing key", input: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send", bad: true},
		{name: "invalid key", input: "abc&other=1", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeWecomRobotID(tt.input)
			if tt.bad && err == nil {
				t.Fatal("expected error")
			}
			if !tt.bad && (err != nil || got != tt.want) {
				t.Fatalf("got %q, err %v", got, err)
			}
		})
	}
}

func TestNormalizeWecomWebhook(t *testing.T) {
	got := normalizeWecomWebhook("abc-123")
	want := "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=abc-123"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
