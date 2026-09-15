package api

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
)

type fakeOIDCProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	claims map[string]interface{}
}

func newFakeOIDCProvider(t *testing.T, claims map[string]interface{}) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeOIDCProvider{key: key, claims: claims}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"issuer": fake.server.URL, "authorization_endpoint": fake.server.URL + "/authorize",
				"token_endpoint": fake.server.URL + "/token", "jwks_uri": fake.server.URL + "/keys",
				"id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			writeJSON(w, http.StatusOK, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: string(jose.RS256), Use: "sig"}}})
		case "/token":
			token := fake.signedToken(t)
			writeJSON(w, http.StatusOK, map[string]interface{}{"access_token": "access", "token_type": "Bearer", "expires_in": 300, "id_token": token})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeOIDCProvider) signedToken(t *testing.T) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
	if err != nil {
		t.Fatal(err)
	}
	claims := map[string]interface{}{
		"iss": f.server.URL, "aud": "warden-client", "sub": "user-123",
		"email": "operator@example.com", "email_verified": true,
		"name": "Test Operator", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	for key, value := range f.claims {
		claims[key] = value
	}
	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func oidcTestHandler(t *testing.T, provider *fakeOIDCProvider, autoProvision string) (*SSOHandler, *db.Store) {
	t.Helper()
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"sso.oidc.enabled": "true", "sso.oidc.issuer_url": provider.server.URL,
		"sso.oidc.client_id": "warden-client", "sso.oidc.client_secret": "warden-secret",
		"sso.oidc.auto_provision": autoProvision,
	} {
		if err := store.SetSetting(key, value); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.CookieSecure = false
	return NewSSOHandler(store, &cfg), store
}

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

func TestOIDCLoginStartsAuthorizationFlow(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, _ := oidcTestHandler(t, provider, "true")
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc", nil)
	recorder := httptest.NewRecorder()

	handler.OIDCLogin(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d", recorder.Code)
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil || location.Host != strings.TrimPrefix(provider.server.URL, "http://") || location.Path != "/authorize" {
		t.Fatalf("unexpected authorization URL %q", recorder.Header().Get("Location"))
	}
	state := location.Query().Get("state")
	if state == "" {
		t.Fatal("authorization URL has no state")
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "oidc_state" || cookies[0].Value != state || !cookies[0].HttpOnly || cookies[0].MaxAge != 300 {
		t.Fatalf("unexpected state cookie: %#v", cookies)
	}
}

func TestOIDCCallbackCreatesUserAndSession(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, store := oidcTestHandler(t, provider, "true")
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?state=known&code=valid", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "known"})
	recorder := httptest.NewRecorder()

	handler.OIDCCallback(recorder, req)

	if recorder.Code != http.StatusTemporaryRedirect || recorder.Header().Get("Location") != "/dashboard" {
		t.Fatalf("unexpected callback response %d %q", recorder.Code, recorder.Header().Get("Location"))
	}
	var authToken string
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "auth_token" {
			authToken = cookie.Value
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
				t.Fatalf("unsafe auth cookie: %#v", cookie)
			}
		}
	}
	if authToken == "" {
		t.Fatal("callback did not set an auth cookie")
	}
	session, err := store.GetSession(authToken)
	if err != nil || session == nil {
		t.Fatalf("session was not stored: %v", err)
	}
	user, err := store.GetUser(session.UserID)
	if err != nil || user.Email != "operator@example.com" || user.SSOProvider != "oidc" {
		t.Fatalf("unexpected OIDC user: %#v, %v", user, err)
	}
}

func TestOIDCCallbackRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		cookie         string
		claims         map[string]interface{}
		autoProvision  string
		allowedDomains string
		wantError      string
	}{
		{name: "missing state cookie", query: "state=known&code=valid", wantError: "invalid_state"},
		{name: "wrong state", query: "state=wrong&code=valid", cookie: "known", wantError: "invalid_state"},
		{name: "provider denial", query: "state=known&error=access_denied", cookie: "known", wantError: "oauth_denied"},
		{name: "missing code", query: "state=known", cookie: "known", wantError: "missing_code"},
		{name: "unverified email", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"email_verified": false}, autoProvision: "true", wantError: "invalid_user_data"},
		{name: "wrong audience", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"aud": "another-client"}, autoProvision: "true", wantError: "invalid_token"},
		{name: "missing subject", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"sub": ""}, autoProvision: "true", wantError: "invalid_user_data"},
		{name: "domain denied", query: "state=known&code=valid", cookie: "known", autoProvision: "true", allowedDomains: "other.example", wantError: "domain_not_allowed"},
		{name: "auto provision disabled", query: "state=known&code=valid", cookie: "known", autoProvision: "false", wantError: "user_not_found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := newFakeOIDCProvider(t, test.claims)
			handler, store := oidcTestHandler(t, provider, test.autoProvision)
			if test.allowedDomains != "" {
				if err := store.SetSetting("sso.oidc.allowed_domains", test.allowedDomains); err != nil {
					t.Fatal(err)
				}
			}
			req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?"+test.query, nil)
			if test.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "oidc_state", Value: test.cookie})
			}
			recorder := httptest.NewRecorder()

			handler.OIDCCallback(recorder, req)

			if recorder.Code != http.StatusTemporaryRedirect || recorder.Header().Get("Location") != "/login?error="+test.wantError {
				t.Fatalf("unexpected response %d %q", recorder.Code, recorder.Header().Get("Location"))
			}
		})
	}
}
