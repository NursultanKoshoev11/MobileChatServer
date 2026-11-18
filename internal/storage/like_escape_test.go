package storage

import "testing"

func TestEscapeLikePattern(t *testing.T) {
	cases := map[string]string{
		"plain":  "plain",
		"50%":    `50\%`,
		"a_b":    `a\_b`,
		`back\s`: `back\\s`,
		`%_\`:    `\%\_\\`,
	}
	for input, want := range cases {
		if got := EscapeLikePattern(input); got != want {
			t.Errorf("EscapeLikePattern(%q) = %q, want %q", input, got, want)
		}
	}
}
