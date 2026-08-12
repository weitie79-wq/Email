package app

import (
	"testing"
)

func TestTelegramHTMLEscape(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"hello <world>&\"quote\"", "hello &lt;world&gt;&amp;&quot;quote&quot;"},
		{"no special chars", "no special chars"},
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"", ""},
		{"中文 subject", "中文 subject"},
		{"1+1=2 & a<b", "1+1=2 &amp; a&lt;b"},
	}
	for _, c := range cases {
		got := telegramHTMLEscape(c.in)
		if got != c.out {
			t.Errorf("telegramHTMLEscape(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestSplitRecipients(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"alice@example.com", []string{"alice@example.com"}},
		{"a@x.com,b@x.com;c@x.com", []string{"a@x.com", "b@x.com", "c@x.com"}},
		{"a@x.com\nb@x.com", []string{"a@x.com", "b@x.com"}},
		{"A@X.COM,B@X.COM", []string{"a@x.com", "b@x.com"}},
		{"", nil},
		{"  ", nil},
		{"a@x.com,a@x.com", []string{"a@x.com"}},
	}
	for _, c := range cases {
		got := splitRecipients(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitRecipients(%q) = %v (len=%d), want %v (len=%d)", c.in, got, len(got), c.want, len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitRecipients(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestGenerateCode(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code := generateCode()
		if len(code) != 6 {
			t.Fatalf("expected length 6, got %d", len(code))
		}
		if seen[code] {
			t.Errorf("duplicate code generated: %s", code)
		}
		seen[code] = true
	}
}

func TestRandInt(t *testing.T) {
	if randInt(0, 10) < 0 || randInt(0, 10) > 10 {
		t.Error("randInt returned value outside range")
	}
	if randInt(5, 5) != 5 {
		t.Error("randInt(5,5) should always return 5")
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"test@example.com", "test@example.com"},
		{"TEST@EXAMPLE.COM", "test@example.com"},
		{"user+tag@gmail.com", "user+tag@gmail.com"},
		{"invalid", "invalid"},
		{"", ""},
		{"@example.com", "@example.com"},
		{"user@", "user@"},
	}
	for _, c := range cases {
		got := normalizeEmail(c.in)
		if got != c.out {
			t.Errorf("normalizeEmail(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}
