package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"strings"
)

// emojiPalette holds exactly 64 single-codepoint emojis with default emoji
// presentation, chosen to look clearly different from each other.
var emojiPalette = [64]string{
	// Animals
	"🐶", "🐱", "🐰", "🦊", "🐻", "🐼", "🐨", "🐯",
	"🐘", "🐮", "🐷", "🐸", "🐵", "🐔", "🐧", "🦉",
	"🐴", "🦄", "🐝", "🦋", "🐌", "🐞", "🐢", "🐍",
	"🐙", "🦀", "🐳", "🦒", "🦓", "🦈",
	// Food
	"🍎", "🍌", "🍇", "🍓", "🍉", "🍍", "🥕", "🌽",
	"🍄", "🥑", "🍒", "🍋", "🍩", "🍕",
	// Objects and nature
	"🚀", "🚗", "🚲", "⚽", "🏀", "🎸", "🎈", "🎁",
	"🔑", "🔔", "💎", "🌈", "⭐", "🌙", "🔥", "🌵",
	"🌻", "🎩", "⛄", "🍀",
}

const codeLength = 4

// emojiCode derives the check code for a giftee: HMAC-SHA256(secret, gifteeNorm),
// with emoji i taken from the palette by the i-th MAC byte.
func emojiCode(secret []byte, gifteeNorm string) []string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(gifteeNorm))
	sum := mac.Sum(nil)
	code := make([]string, codeLength)
	for i := range code {
		code[i] = emojiPalette[int(sum[i])%len(emojiPalette)]
	}
	return code
}

// newRoomSecret returns a 32-byte secret for which every name in norms gets a
// distinct check code.
func newRoomSecret(norms []string) ([]byte, error) {
	for {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, err
		}
		if codesDistinct(secret, norms) {
			return secret, nil
		}
	}
}

func codesDistinct(secret []byte, norms []string) bool {
	seen := make(map[string]struct{}, len(norms))
	for _, n := range norms {
		c := strings.Join(emojiCode(secret, n), "")
		if _, dup := seen[c]; dup {
			return false
		}
		seen[c] = struct{}{}
	}
	return true
}
