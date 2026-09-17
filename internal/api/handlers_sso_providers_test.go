package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
)

const (
	testOIDCNonce    = "test-nonce"
	testOIDCVerifier = "test-pkce-verifier"
)

type fakeOIDCProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newFakeOIDCProvider(t *testing.T, tokenClaims map[string]any) *fakeOIDCProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeOIDCProvider{key: key}
	fake.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]any{
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
			claims := map[string]any{
				"iss": fake.server.URL, "aud": "warden-client", "sub": "user-123",
				"email": "operator@example.com", "email_verified": true,
				"name": "Test Operator", "nonce": testOIDCNonce,
				"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
			}
			for key, value := range tokenClaims {
				claims[key] = value
			}
			signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: fake.key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
			if err != nil {
				t.Fatal(err)
			}
			token, err := jwt.Signed(signer).Claims(claims).Serialize()
			if err != nil {
				t.Fatal(err)
			}
			writeJSON(w, http.StatusOK, map[string]any{"access_token": "access", "token_type": "Bearer", "expires_in": 300, "id_token": token})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(fake.server.Close)
	return fake
}

func providerRequest(method, path string, body []byte, id string, role string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	contextWithRole := context.WithValue(request.Context(), contextKeyUserRole, role)
	if id == "" {
		return request.WithContext(contextWithRole)
	}
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", id)
	return request.WithContext(context.WithValue(contextWithRole, chi.RouteCtxKey, routeContext))
}

func TestProviderHandlersCRUDAndPublicList(t *testing.T) {
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewSSOHandler(store, &config.Config{})

	body := []byte(`{"template":"oidc","name":"Company SSO","issuerUrl":"https://id.example.com","clientId":"warden","clientSecret":"secret","allowedDomains":"example.com","autoProvision":true,"enabled":true}`)
	recorder := httptest.NewRecorder()
	handler.CreateProvider(recorder, providerRequest(http.MethodPost, "/api/sso/providers", body, "", RoleAdmin))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var created providerResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !created.SecretConfigured {
		t.Fatalf("unexpected response: %#v", created)
	}
	if bytes.Contains(recorder.Body.Bytes(), []byte("secret\"")) {
		t.Fatal("response exposed the client secret")
	}

	publicRecorder := httptest.NewRecorder()
	handler.PublicProviders(publicRecorder, httptest.NewRequest(http.MethodGet, "/api/auth/sso/providers", nil))
	if publicRecorder.Code != http.StatusOK || !bytes.Contains(publicRecorder.Body.Bytes(), []byte(`"name":"Company SSO"`)) {
		t.Fatalf("unexpected public response: %d %s", publicRecorder.Code, publicRecorder.Body.String())
	}

	update := []byte(`{"template":"oidc","name":"Renamed SSO","issuerUrl":"https://id.example.com","clientId":"warden","clientSecret":"","autoProvision":false,"enabled":false}`)
	recorder = httptest.NewRecorder()
	handler.UpdateProvider(recorder, providerRequest(http.MethodPut, "/api/sso/providers/"+created.ID, update, created.ID, RoleAdmin))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	stored, _ := store.GetSSOProvider(created.ID)
	if stored.Name != "Renamed SSO" || stored.ClientSecret != "secret" || stored.Enabled {
		t.Fatalf("unexpected stored provider: %#v", stored)
	}
}

func TestProviderHandlersRequireAdmin(t *testing.T) {
	store, _ := db.NewStore(db.NewTestConfig())
	handler := NewSSOHandler(store, &config.Config{})
	recorder := httptest.NewRecorder()
	handler.ListProviders(recorder, providerRequest(http.MethodGet, "/api/sso/providers", nil, "", RoleEditor))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", recorder.Code)
	}
}

func TestNormalizeProvider(t *testing.T) {
	provider, err := normalizeProvider(providerInput{Template: "google", Name: " Google ", IssuerURL: "ignored", ClientID: "client", ClientSecret: "secret", AllowedDomains: "example.com"})
	if err != nil || provider.IssuerURL != "https://accounts.google.com" || provider.Name != "Google" {
		t.Fatalf("unexpected normalized provider: %#v, %v", provider, err)
	}
	if _, err := normalizeProvider(providerInput{Template: "oidc", Name: "Bad", IssuerURL: "javascript:alert(1)", ClientID: "client"}); err == nil {
		t.Fatal("expected an invalid issuer error")
	}
	if _, err := normalizeProvider(providerInput{Template: "oidc", Name: "Draft", IssuerURL: "https://id.example.com"}); err != nil {
		t.Fatalf("disabled drafts should not require credentials: %v", err)
	}
	if _, err := normalizeProvider(providerInput{Template: "oidc", Name: "Enabled", IssuerURL: "https://id.example.com", Enabled: true}); err == nil {
		t.Fatal("enabled providers must require credentials")
	}
}

func TestProviderLoginAndCallback(t *testing.T) {
	fake := newFakeOIDCProvider(t, nil)
	store, err := db.NewStore(db.NewTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	provider := db.SSOProvider{ID: "idp-company", Template: "oidc", Name: "Company SSO", IssuerURL: fake.server.URL, ClientID: "warden-client", ClientSecret: "warden-secret", AutoProvision: true, Enabled: true}
	if err := store.CreateSSOProvider(provider); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.CookieSecure = false
	handler := NewSSOHandler(store, &cfg)

	login := providerRequest(http.MethodGet, "http://warden.test/api/auth/sso/idp-company", nil, provider.ID, "")
	loginRecorder := httptest.NewRecorder()
	handler.ProviderLogin(loginRecorder, login)
	if loginRecorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected login redirect, got %d", loginRecorder.Code)
	}
	if location := loginRecorder.Header().Get("Location"); !bytes.Contains([]byte(location), []byte("code_challenge_method=S256")) {
		t.Fatalf("login does not use PKCE: %s", location)
	}

	callback := providerRequest(http.MethodGet, "http://warden.test/api/auth/sso/idp-company/callback?state=known&code=valid", nil, provider.ID, "")
	callback.AddCookie(&http.Cookie{Name: providerCookieName("state", provider.ID), Value: "known"})
	callback.AddCookie(&http.Cookie{Name: providerCookieName("nonce", provider.ID), Value: testOIDCNonce})
	callback.AddCookie(&http.Cookie{Name: providerCookieName("pkce", provider.ID), Value: testOIDCVerifier})
	callbackRecorder := httptest.NewRecorder()
	handler.ProviderCallback(callbackRecorder, callback)
	if callbackRecorder.Header().Get("Location") != "/dashboard" {
		t.Fatalf("unexpected callback redirect: %s", callbackRecorder.Header().Get("Location"))
	}
	var token string
	for _, cookie := range callbackRecorder.Result().Cookies() {
		if cookie.Name == "auth_token" {
			token = cookie.Value
		}
	}
	session, err := store.GetSession(token)
	if err != nil || session == nil {
		t.Fatalf("session was not created: %v", err)
	}
	user, _ := store.GetUser(session.UserID)
	if user.SSOProvider != provider.ID || user.Role != RoleViewer {
		t.Fatalf("unexpected provisioned user: %#v", user)
	}
}

func TestProviderCallbackRejectsWrongProviderState(t *testing.T) {
	store, _ := db.NewStore(db.NewTestConfig())
	cfg := config.Default()
	handler := NewSSOHandler(store, &cfg)
	request := providerRequest(http.MethodGet, "http://warden.test/api/auth/sso/idp-two/callback?state=known&code=valid", nil, "idp-two", "")
	request.AddCookie(&http.Cookie{Name: providerCookieName("state", "idp-one"), Value: "known"})
	recorder := httptest.NewRecorder()
	handler.ProviderCallback(recorder, request)
	if recorder.Header().Get("Location") != "/login?error=invalid_state" {
		t.Fatalf("unexpected redirect: %s", recorder.Header().Get("Location"))
	}
}

func TestProviderCallbackRejectsInvalidIdentity(t *testing.T) {
	tests := []struct {
		name          string
		claims        map[string]any
		domains       string
		autoProvision bool
		wantError     string
	}{
		{name: "unverified email", claims: map[string]any{"email_verified": false}, autoProvision: true, wantError: "invalid_user_data"},
		{name: "wrong audience", claims: map[string]any{"aud": "another-client"}, autoProvision: true, wantError: "invalid_token"},
		{name: "wrong nonce", claims: map[string]any{"nonce": "replayed"}, autoProvision: true, wantError: "invalid_token"},
		{name: "domain denied", domains: "other.example", autoProvision: true, wantError: "domain_not_allowed"},
		{name: "auto provisioning disabled", autoProvision: false, wantError: "user_not_found"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeOIDCProvider(t, test.claims)
			store, _ := db.NewStore(db.NewTestConfig())
			provider := db.SSOProvider{ID: "idp-company", Template: "oidc", Name: "Company", IssuerURL: fake.server.URL, ClientID: "warden-client", ClientSecret: "secret", AllowedDomains: test.domains, AutoProvision: test.autoProvision, Enabled: true}
			if err := store.CreateSSOProvider(provider); err != nil {
				t.Fatal(err)
			}
			cfg := config.Default()
			handler := NewSSOHandler(store, &cfg)
			request := providerRequest(http.MethodGet, "http://warden.test/api/auth/sso/idp-company/callback?state=known&code=valid", nil, provider.ID, "")
			request.AddCookie(&http.Cookie{Name: providerCookieName("state", provider.ID), Value: "known"})
			request.AddCookie(&http.Cookie{Name: providerCookieName("nonce", provider.ID), Value: testOIDCNonce})
			request.AddCookie(&http.Cookie{Name: providerCookieName("pkce", provider.ID), Value: testOIDCVerifier})
			recorder := httptest.NewRecorder()
			handler.ProviderCallback(recorder, request)
			if recorder.Header().Get("Location") != "/login?error="+test.wantError {
				t.Fatalf("unexpected redirect: %s", recorder.Header().Get("Location"))
			}
		})
	}
}

func TestGoogleProviderAddsHostedDomainHint(t *testing.T) {
	fake := newFakeOIDCProvider(t, nil)
	store, _ := db.NewStore(db.NewTestConfig())
	provider := db.SSOProvider{ID: "idp-google", Template: "google", Name: "Google", IssuerURL: fake.server.URL, ClientID: "warden-client", ClientSecret: "secret", AllowedDomains: "example.com, other.example", Enabled: true}
	if err := store.CreateSSOProvider(provider); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	handler := NewSSOHandler(store, &cfg)
	recorder := httptest.NewRecorder()
	handler.ProviderLogin(recorder, providerRequest(http.MethodGet, "http://warden.test/api/auth/sso/idp-google", nil, provider.ID, ""))
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil || location.Query().Get("hd") != "example.com" {
		t.Fatalf("missing hosted-domain hint: %s", recorder.Header().Get("Location"))
	}
}

func TestProviderConnectionTestUsesDiscovery(t *testing.T) {
	fake := newFakeOIDCProvider(t, nil)
	store, _ := db.NewStore(db.NewTestConfig())
	provider := db.SSOProvider{ID: "idp-test", Template: "oidc", Name: "Test", IssuerURL: fake.server.URL, ClientID: "client", ClientSecret: "secret"}
	if err := store.CreateSSOProvider(provider); err != nil {
		t.Fatal(err)
	}
	handler := NewSSOHandler(store, &config.Config{})
	recorder := httptest.NewRecorder()
	handler.TestProvider(recorder, providerRequest(http.MethodPost, "/api/sso/providers/idp-test/test", nil, provider.ID, RoleAdmin))
	if recorder.Code != http.StatusOK || !bytes.Contains(recorder.Body.Bytes(), []byte(`"valid":true`)) {
		t.Fatalf("unexpected test response: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestOIDCIdentityHelpers(t *testing.T) {
	if !emailDomainAllowed("person@EXAMPLE.com", "other.example, example.com") {
		t.Fatal("expected case-insensitive domain match")
	}
	if emailDomainAllowed("person@evil.example", "example.com") {
		t.Fatal("unexpected domain match")
	}
	first := oidcSubjectID("https://id.example/one", "subject")
	second := oidcSubjectID("https://id.example/two", "subject")
	if first == second {
		t.Fatal("issuer must be part of the stored identity")
	}
}
