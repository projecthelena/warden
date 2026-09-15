package api

import "testing"

func TestEmailDomainAllowed(t *testing.T) {
	tests := []struct {
		email, domains string
		want           bool
	}{
		{"user@example.com", "", true},
		{"user@example.com", "example.com", true},
		{"user@EXAMPLE.com", "other.test, example.com", true},
		{"user@evil.test", "example.com", false},
		{"invalid", "example.com", false},
	}
	for _, test := range tests {
		if got := emailDomainAllowed(test.email, test.domains); got != test.want {
			t.Errorf("emailDomainAllowed(%q, %q) = %v, want %v", test.email, test.domains, got, test.want)
		}
	}
}

func TestOIDCSubjectIDIncludesIssuer(t *testing.T) {
	first := oidcSubjectID("https://id.example.com/realms/first", "user-123")
	second := oidcSubjectID("https://id.example.com/realms/second", "user-123")
	if first == second {
		t.Fatal("subjects from different issuers must not share an identity")
	}

	if got := oidcSubjectID("https://id.example.com/realms/first/", "user-123"); got != first {
		t.Fatalf("trailing slash changed issuer identity: %q != %q", got, first)
	}
}
