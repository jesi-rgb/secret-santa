package main

import (
	"errors"
	"testing"
)

// checkAssignment fails unless p is a permutation of [0, n) with no fixed
// points and no excluded giver→giftee pair.
func checkAssignment(t *testing.T, n int, excluded [][]bool, p []int) {
	t.Helper()
	if len(p) != n {
		t.Fatalf("n=%d: got length %d", n, len(p))
	}
	seen := make([]bool, n)
	for i, v := range p {
		if v < 0 || v >= n {
			t.Fatalf("n=%d: out of range value %d", n, v)
		}
		if v == i {
			t.Fatalf("n=%d: fixed point at %d: %v", n, i, p)
		}
		if excluded != nil && excluded[i][v] {
			t.Fatalf("n=%d: excluded pair %d→%d: %v", n, i, v, p)
		}
		if seen[v] {
			t.Fatalf("n=%d: not a permutation: %v", n, p)
		}
		seen[v] = true
	}
}

// exclusionMatrix builds an n×n matrix with both directions of each pair set.
func exclusionMatrix(n int, pairs ...[2]int) [][]bool {
	m := make([][]bool, n)
	for i := range m {
		m[i] = make([]bool, n)
	}
	for _, p := range pairs {
		m[p[0]][p[1]], m[p[1]][p[0]] = true, true
	}
	return m
}

func TestDrawAssignment(t *testing.T) {
	for n := 3; n <= 20; n++ {
		for range 200 {
			p, err := drawAssignment(n, nil)
			if err != nil {
				t.Fatal(err)
			}
			checkAssignment(t, n, nil, p)
		}
	}
}

func TestDrawAssignmentTooSmall(t *testing.T) {
	for _, n := range []int{0, 1} {
		if _, err := drawAssignment(n, nil); err == nil {
			t.Fatalf("n=%d: expected error", n)
		}
	}
}

func TestDrawAssignmentExclusions(t *testing.T) {
	couples := exclusionMatrix(6, [2]int{0, 1}, [2]int{2, 3}, [2]int{4, 5})
	for range 200 {
		p, err := drawAssignment(6, couples)
		if err != nil {
			t.Fatal(err)
		}
		checkAssignment(t, 6, couples, p)
	}
}

// Only the cycle i→i+1 is allowed: rejection sampling almost never hits it,
// so this exercises the matching fallback.
func TestDrawAssignmentSingleValidDraw(t *testing.T) {
	const n = 9
	excluded := make([][]bool, n)
	for i := range excluded {
		excluded[i] = make([]bool, n)
		for j := range excluded[i] {
			excluded[i][j] = j != (i+1)%n
		}
	}
	p, err := drawAssignment(n, excluded)
	if err != nil {
		t.Fatal(err)
	}
	checkAssignment(t, n, excluded, p)
}

func TestDrawAssignmentImpossible(t *testing.T) {
	cases := map[string]struct {
		n        int
		excluded [][]bool
	}{
		// The third person must gift and receive from the same one of the pair.
		"pair among three":       {3, exclusionMatrix(3, [2]int{0, 1})},
		"excluded from everyone": {4, exclusionMatrix(4, [2]int{0, 1}, [2]int{0, 2}, [2]int{0, 3})},
		// 0 and 1 can each only gift 4.
		"shared last option": {5, exclusionMatrix(5, [2]int{0, 1}, [2]int{0, 2}, [2]int{0, 3}, [2]int{1, 2}, [2]int{1, 3})},
	}
	for name, c := range cases {
		if _, err := drawAssignment(c.n, c.excluded); !errors.Is(err, ErrImpossibleDraw) {
			t.Errorf("%s: got %v, want ErrImpossibleDraw", name, err)
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := map[string]string{
		"  José  María ": "jose maria",
		"ANA":            "ana",
		"Zoë\tO'Brien":   "zoe o'brien",
		"Ñandú":          "nandu",
		"   ":            "",
	}
	for in, want := range cases {
		if got := normalizeName(in); got != want {
			t.Errorf("normalizeName(%q) = %q, want %q", in, got, want)
		}
	}
	if normalizeName("  José  María ") != normalizeName("jose maria") {
		t.Fatal("expected equal normalized names")
	}
}

func TestNewRoomID(t *testing.T) {
	for range 100 {
		id, err := newRoomID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 10 {
			t.Fatalf("bad length: %q", id)
		}
		for _, c := range id {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				t.Fatalf("bad char in %q", id)
			}
		}
	}
}
