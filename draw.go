package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// randIntn returns a uniformly random integer in [0, n) using crypto/rand.
func randIntn(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// ErrImpossibleDraw means no assignment avoids every self-gift and exclusion.
var ErrImpossibleDraw = errors.New("no draw satisfies the exclusions")

// maxDrawAttempts bounds rejection sampling in drawAssignment. Without
// exclusions about e attempts are expected; a few exclusions per person keep
// the expected count far below this.
const maxDrawAttempts = 5000

// shuffle permutes p uniformly at random (Fisher–Yates).
func shuffle(p []int) error {
	for i := len(p) - 1; i > 0; i-- {
		j, err := randIntn(i + 1)
		if err != nil {
			return err
		}
		p[i], p[j] = p[j], p[i]
	}
	return nil
}

// drawAssignment returns a random permutation p of [0, n) where participant i
// gifts participant p[i], p[i] != i, and excluded[i][p[i]] is false.
// excluded is nil (no exclusions) or n×n.
//
// It returns ErrImpossibleDraw when no such permutation exists. Otherwise it
// samples uniformly by rejection (shuffle until valid). If that keeps failing
// because the exclusions leave very few valid draws, it returns a randomized
// matching instead: valid, but not exactly uniform.
func drawAssignment(n int, excluded [][]bool) ([]int, error) {
	if n < 2 {
		return nil, errors.New("a draw needs at least 2 participants")
	}
	allowed := func(i, j int) bool { return i != j && (excluded == nil || !excluded[i][j]) }

	fallback, err := randomMatching(n, allowed)
	if err != nil {
		return nil, err
	}
	p := make([]int, n)
	for range maxDrawAttempts {
		for i := range p {
			p[i] = i
		}
		if err := shuffle(p); err != nil {
			return nil, err
		}
		ok := true
		for i, v := range p {
			if !allowed(i, v) {
				ok = false
				break
			}
		}
		if ok {
			return p, nil
		}
	}
	return fallback, nil
}

// randomMatching finds an assignment using only allowed giver→giftee pairs, by
// augmenting paths (Kuhn's algorithm) over randomly ordered givers and
// candidates. It returns ErrImpossibleDraw if none exists.
func randomMatching(n int, allowed func(i, j int) bool) ([]int, error) {
	candidates := make([][]int, n)
	for i := range n {
		for j := range n {
			if allowed(i, j) {
				candidates[i] = append(candidates[i], j)
			}
		}
		if err := shuffle(candidates[i]); err != nil {
			return nil, err
		}
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if err := shuffle(order); err != nil {
		return nil, err
	}

	giverOf := make([]int, n) // giftee → giver; -1 while unassigned
	for j := range giverOf {
		giverOf[j] = -1
	}
	visited := make([]bool, n)
	var augment func(i int) bool
	augment = func(i int) bool {
		for _, j := range candidates[i] {
			if visited[j] {
				continue
			}
			visited[j] = true
			if giverOf[j] < 0 || augment(giverOf[j]) {
				giverOf[j] = i
				return true
			}
		}
		return false
	}
	for _, i := range order {
		clear(visited)
		if !augment(i) {
			return nil, ErrImpossibleDraw
		}
	}
	p := make([]int, n)
	for j, i := range giverOf {
		p[i] = j
	}
	return p, nil
}

// normalizeName folds a name for matching: Unicode NFD, combining marks
// stripped, lowercased, internal whitespace collapsed, trimmed.
func normalizeName(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)))
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	return strings.Join(strings.Fields(strings.ToLower(out)), " ")
}

const roomIDAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// newRoomID returns 10 characters from [a-z0-9], uniformly distributed.
func newRoomID() (string, error) {
	const n = 10
	// 252 is the largest multiple of 36 that fits in a byte; reject above it.
	const limit = 252
	out := make([]byte, 0, n)
	buf := make([]byte, 16)
	for len(out) < n {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b >= limit {
				continue
			}
			out = append(out, roomIDAlphabet[int(b)%len(roomIDAlphabet)])
			if len(out) == n {
				break
			}
		}
	}
	return string(out), nil
}

// newToken returns 32 random bytes encoded as unpadded base64url.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
