package main

import "testing"

func TestDerangement(t *testing.T) {
	for n := 3; n <= 20; n++ {
		for range 200 {
			p, err := derangement(n)
			if err != nil {
				t.Fatal(err)
			}
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
				if seen[v] {
					t.Fatalf("n=%d: not a permutation: %v", n, p)
				}
				seen[v] = true
			}
		}
	}
}

func TestDerangementTooSmall(t *testing.T) {
	for _, n := range []int{0, 1} {
		if _, err := derangement(n); err == nil {
			t.Fatalf("n=%d: expected error", n)
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
