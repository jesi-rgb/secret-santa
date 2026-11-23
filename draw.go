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

// derangement returns a uniformly random permutation p of [0, n) with
// p[i] != i for every i. Participant i gifts participant p[i].
// It shuffles with Fisher–Yates and rejects shuffles that have fixed points;
// the expected number of attempts is about e.
func derangement(n int) ([]int, error) {
	if n < 2 {
		return nil, errors.New("derangement needs at least 2 elements")
	}
	p := make([]int, n)
	for {
		for i := range p {
			p[i] = i
		}
		for i := n - 1; i > 0; i-- {
			j, err := randIntn(i + 1)
			if err != nil {
				return nil, err
			}
			p[i], p[j] = p[j], p[i]
		}
		ok := true
		for i, v := range p {
			if i == v {
				ok = false
				break
			}
		}
		if ok {
			return p, nil
		}
	}
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
