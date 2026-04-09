package locale

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()
	if g := Normalize("zh-CN"); g != "zh-CN" {
		t.Fatalf("zh-CN: got %q", g)
	}
	if g := Normalize("zh_HK"); g != "zh-CN" {
		t.Fatalf("zh_*: got %q", g)
	}
	if g := Normalize("en-US"); g != "en" {
		t.Fatalf("en-US: got %q", g)
	}
}

func TestMessageFallback(t *testing.T) {
	t.Parallel()
	if s := Message("zh-CN", ErrStoreNotInit); s == "" || s == ErrStoreNotInit {
		t.Fatalf("expected Chinese message, got %q", s)
	}
	if s := Message("xx", ErrStoreNotInit); s == "" {
		t.Fatal("expected English fallback")
	}
}
