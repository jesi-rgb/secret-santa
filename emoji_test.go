package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEmojiPalette(t *testing.T) {
	seen := make(map[string]bool, len(emojiPalette))
	for i, e := range emojiPalette {
		if utf8.RuneCountInString(e) != 1 {
			t.Errorf("palette[%d] = %q is not a single codepoint", i, e)
		}
		if seen[e] {
			t.Errorf("palette[%d] = %q is duplicated", i, e)
		}
		seen[e] = true
	}
	if len(seen) != 64 {
		t.Fatalf("palette has %d distinct emojis, want 64", len(seen))
	}
}

func TestEmojiCodeStable(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	a := emojiCode(secret, "ana")
	b := emojiCode(secret, "ana")
	if len(a) != codeLength {
		t.Fatalf("code length %d", len(a))
	}
	if strings.Join(a, "") != strings.Join(b, "") {
		t.Fatalf("code not stable: %v vs %v", a, b)
	}
	other := emojiCode([]byte("another secret, another code...."), "ana")
	if strings.Join(a, "") == strings.Join(other, "") {
		t.Fatalf("different secrets produced the same code %v", a)
	}
}

func TestNewRoomSecretDistinctCodes(t *testing.T) {
	norms := make([]string, 100)
	for i := range norms {
		norms[i] = fmt.Sprintf("person %d", i)
	}
	for range 20 {
		secret, err := newRoomSecret(norms)
		if err != nil {
			t.Fatal(err)
		}
		if len(secret) != 32 {
			t.Fatalf("secret length %d", len(secret))
		}
		seen := make(map[string]bool)
		for _, n := range norms {
			c := strings.Join(emojiCode(secret, n), "")
			if seen[c] {
				t.Fatalf("duplicate code %s", c)
			}
			seen[c] = true
		}
	}
}

func TestCodesDistinctDetectsCollision(t *testing.T) {
	if codesDistinct([]byte("s"), []string{"ana", "ana"}) {
		t.Fatal("identical names must collide")
	}
}
