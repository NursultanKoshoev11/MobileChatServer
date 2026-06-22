package config

import "testing"

func TestOriginListContainsWildcard(t *testing.T) {
	if !originListContainsWildcard("https://example.com, *") {
		t.Fatal("expected wildcard to be detected")
	}
	if originListContainsWildcard("https://example.com,https://admin.example.com") {
		t.Fatal("did not expect wildcard")
	}
}
