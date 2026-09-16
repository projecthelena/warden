package api

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
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

const (
	testOIDCNonce    = "test-nonce"
	testOIDCVerifier = "test-pkce-verifier"
)

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
			if err := r.ParseForm(); err != nil || r.Form.Get("code_verifier") != testOIDCVerifier {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
				return
			}
			if r.Form.Get("code") == "exchange-fails" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
				return
			}
			if r.Form.Get("code") == "missing-token" {
				writeJSON(w, http.StatusOK, map[string]interface{}{"access_token": "access", "token_type": "Bearer", "expires_in": 300})
				return
			}
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
		"name": "Test Operator", "nonce": testOIDCNonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
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

func addOIDCCookies(req *http.Request, state string) {
	req.AddCookie(&http.Cookie{Name: oidcStateCookie, Value: state})
	req.AddCookie(&http.Cookie{Name: oidcNonceCookie, Value: testOIDCNonce})
	req.AddCookie(&http.Cookie{Name: oidcPKCECookie, Value: testOIDCVerifier})
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
	if location.Query().Get("nonce") == "" || location.Query().Get("code_challenge") == "" || location.Query().Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL lacks nonce or PKCE: %q", location.RawQuery)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("expected state, nonce, and PKCE cookies, got %#v", cookies)
	}
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		if !cookie.HttpOnly || cookie.MaxAge != 300 || cookie.SameSite != http.SameSiteLaxMode {
			t.Fatalf("unsafe OIDC cookie: %#v", cookie)
		}
		values[cookie.Name] = cookie.Value
	}
	if values[oidcStateCookie] != state || values[oidcNonceCookie] != location.Query().Get("nonce") || values[oidcPKCECookie] == "" {
		t.Fatalf("OIDC cookies do not match authorization request: %#v", values)
	}
}

func TestOIDCLoginUsesSecureTransientCookies(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, _ := oidcTestHandler(t, provider, "true")
	handler.config.CookieSecure = true
	recorder := httptest.NewRecorder()

	handler.OIDCLogin(recorder, httptest.NewRequest(http.MethodGet, "https://warden.test/api/auth/sso/oidc", nil))

	for _, cookie := range recorder.Result().Cookies() {
		if !cookie.Secure {
			t.Fatalf("OIDC cookie %s was not Secure", cookie.Name)
		}
	}
}

func TestOIDCCallbackCreatesUserAndSession(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, store := oidcTestHandler(t, provider, "true")
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?state=known&code=valid", nil)
	addOIDCCookies(req, "known")
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
	cleared := map[string]bool{}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.MaxAge == -1 {
			cleared[cookie.Name] = true
		}
	}
	for _, name := range []string{oidcStateCookie, oidcNonceCookie, oidcPKCECookie} {
		if !cleared[name] {
			t.Fatalf("callback did not clear %s", name)
		}
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

func TestOIDCCallbackAllowsExistingIdentityWhenAutoProvisionIsDisabled(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, store := oidcTestHandler(t, provider, "true")
	callback := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?state=known&code=valid", nil)
		addOIDCCookies(req, "known")
		recorder := httptest.NewRecorder()
		handler.OIDCCallback(recorder, req)
		return recorder
	}
	if recorder := callback(); recorder.Header().Get("Location") != "/dashboard" {
		t.Fatalf("initial provisioning failed: %q", recorder.Header().Get("Location"))
	}
	if err := store.SetSetting("sso.oidc.auto_provision", "false"); err != nil {
		t.Fatal(err)
	}
	if recorder := callback(); recorder.Header().Get("Location") != "/dashboard" {
		t.Fatalf("existing identity was rejected: %q", recorder.Header().Get("Location"))
	}
}

func TestOIDCCallbackDoesNotAutoLinkPasswordAccount(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, store := oidcTestHandler(t, provider, "true")
	user, err := store.FindOrCreateSSOUser("google", "google-operator", "operator@example.com", "Operator", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserPassword(user.ID, "password123!"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?state=known&code=valid", nil)
	addOIDCCookies(req, "known")
	recorder := httptest.NewRecorder()

	handler.OIDCCallback(recorder, req)

	if recorder.Header().Get("Location") != "/login?error=account_exists_link_required" {
		t.Fatalf("password account was not protected: %q", recorder.Header().Get("Location"))
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
		{name: "token exchange fails", query: "state=known&code=exchange-fails", cookie: "known", autoProvision: "true", wantError: "token_exchange_failed"},
		{name: "missing ID token", query: "state=known&code=missing-token", cookie: "known", autoProvision: "true", wantError: "invalid_token"},
		{name: "unverified email", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"email_verified": false}, autoProvision: "true", wantError: "invalid_user_data"},
		{name: "wrong audience", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"aud": "another-client"}, autoProvision: "true", wantError: "invalid_token"},
		{name: "wrong issuer", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"iss": "https://attacker.example"}, autoProvision: "true", wantError: "invalid_token"},
		{name: "expired token", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"exp": time.Now().Add(-time.Hour).Unix()}, autoProvision: "true", wantError: "invalid_token"},
		{name: "missing subject", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"sub": ""}, autoProvision: "true", wantError: "invalid_user_data"},
		{name: "missing email", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"email": ""}, autoProvision: "true", wantError: "invalid_user_data"},
		{name: "wrong nonce", query: "state=known&code=valid", cookie: "known", claims: map[string]interface{}{"nonce": "replayed"}, autoProvision: "true", wantError: "invalid_token"},
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
				addOIDCCookies(req, test.cookie)
			}
			recorder := httptest.NewRecorder()

			handler.OIDCCallback(recorder, req)

			if recorder.Code != http.StatusTemporaryRedirect || recorder.Header().Get("Location") != "/login?error="+test.wantError {
				t.Fatalf("unexpected response %d %q", recorder.Code, recorder.Header().Get("Location"))
			}
		})
	}
}

func TestOIDCCallbackRequiresAllTransientCookies(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, _ := oidcTestHandler(t, provider, "true")
	cookies := []http.Cookie{
		{Name: oidcStateCookie, Value: "known"},
		{Name: oidcNonceCookie, Value: testOIDCNonce},
		{Name: oidcPKCECookie, Value: testOIDCVerifier},
	}
	for missing := range cookies {
		t.Run(cookies[missing].Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc/callback?state=known&code=valid", nil)
			for i := range cookies {
				if i != missing {
					req.AddCookie(&cookies[i])
				}
			}
			recorder := httptest.NewRecorder()
			handler.OIDCCallback(recorder, req)
			if recorder.Header().Get("Location") != "/login?error=invalid_state" {
				t.Fatalf("missing %s redirected to %q", cookies[missing].Name, recorder.Header().Get("Location"))
			}
		})
	}
}

func TestOIDCLoginRejectsInvalidHost(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, _ := oidcTestHandler(t, provider, "true")
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc", nil)
	req.Host = "attacker.example/path"
	recorder := httptest.NewRecorder()

	handler.OIDCLogin(recorder, req)

	if recorder.Header().Get("Location") != "/login?error=invalid_request" {
		t.Fatalf("invalid host redirected to %q", recorder.Header().Get("Location"))
	}
}

func TestOIDCLoginRequiresCompleteConfiguration(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	handler := NewSSOHandler(store, &cfg)
	req := httptest.NewRequest(http.MethodGet, "http://warden.test/api/auth/sso/oidc", nil)
	recorder := httptest.NewRecorder()

	handler.OIDCLogin(recorder, req)

	if recorder.Header().Get("Location") != "/login?error=sso_not_configured" {
		t.Fatalf("incomplete configuration redirected to %q", recorder.Header().Get("Location"))
	}
}

func TestSSOStatusPublishesOIDCOnlyWhenConfigurationIsComplete(t *testing.T) {
	provider := newFakeOIDCProvider(t, nil)
	handler, store := oidcTestHandler(t, provider, "true")
	if err := store.SetSetting("sso.oidc.provider_name", "Company SSO"); err != nil {
		t.Fatal(err)
	}
	readStatus := func() map[string]interface{} {
		recorder := httptest.NewRecorder()
		handler.GetSSOStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/sso/status", nil))
		var status map[string]interface{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		return status
	}
	status := readStatus()
	if status["oidc"] != true || status["oidcProviderName"] != "Company SSO" {
		t.Fatalf("complete provider was not published: %#v", status)
	}
	if err := store.SetSetting("sso.oidc.client_secret", ""); err != nil {
		t.Fatal(err)
	}
	if status = readStatus(); status["oidc"] != false {
		t.Fatalf("incomplete provider was published: %#v", status)
	}
}
