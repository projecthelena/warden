package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clusteruptime/clusteruptime/internal/config"
	"github.com/clusteruptime/clusteruptime/internal/db"
	"github.com/clusteruptime/clusteruptime/internal/uptime"
)

// TestAPIKeyIntegrationFlow simulates the full user journey:
// 1. Login (setup default admin)
// 2. Create API Key
// 3. Use API Key to Create Group
// 4. Use API Key to Create Monitor
func TestAPIKeyIntegrationFlow(t *testing.T) {
	// 1. Setup Server
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, _ := db.NewStore(dbPath)
	manager := uptime.NewManager(store)
	// Use default config for testing
	cfg := config.Default()
	router := NewRouter(manager, store, &cfg)

	ts := httptest.NewServer(router)
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	client := ts.Client()
	client.Jar = jar

	// Helper for requests
	baseURL := ts.URL + "/api"

	// 1.1 Edge Case: Short Password
	badSetupPayload := map[string]interface{}{
		"username": "admin",
		"password": "123", // Too short
		"timezone": "UTC",
	}
	badBody, _ := json.Marshal(badSetupPayload)
	resp, err := client.Post(baseURL+"/setup", "application/json", bytes.NewBuffer(badBody))
	if err != nil {
		t.Fatalf("Bad setup req failed: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("Expected 400 for short password, got %d", resp.StatusCode)
	}

	// 1.5. Perform Setup (Create Admin User)
	setupPayload := map[string]interface{}{
		"username":       "admin",
		"password":       "Password123!", // Strong Password
		"timezone":       "UTC",
		"createDefaults": true,
	}
	setupBody, _ := json.Marshal(setupPayload)
	resp, err = client.Post(baseURL+"/setup", "application/json", bytes.NewBuffer(setupBody))
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if resp.StatusCode != 200 {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		t.Fatalf("Setup failed: %d %s", resp.StatusCode, buf.String())
	}

	// 1.5b Verify Status is now true
	resp, err = client.Get(baseURL + "/setup/status")
	if err != nil {
		t.Fatalf("Failed to check setup status: %v", err)
	}
	var statusAfter map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&statusAfter); err != nil {
		t.Fatalf("Failed to decode status: %v", err)
	}
	if !statusAfter["isSetup"] {
		t.Fatal("Expected isSetup to be true after setup")
	}

	// 1.6 Edge Case: Setup Again (Should fail with 403 Forbidden)
	resp, err = client.Post(baseURL+"/setup", "application/json", bytes.NewBuffer(setupBody))
	if err != nil {
		t.Fatalf("Re-setup req failed: %v", err)
	}
	if resp.StatusCode != 403 {
		t.Fatalf("Expected 403 Forbidden for re-setup, got %d", resp.StatusCode)
	}

	// 2. Login as Admin
	// Note: NewStore defaults admin/password if empty - NO LONGER TRUE. Setup required.
	loginPayload := map[string]string{"username": "admin", "password": "Password123!"}
	loginBody, _ := json.Marshal(loginPayload)
	resp, err = client.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Login status: %d", resp.StatusCode)
	}
	// Cookies are handled by jar if configured, creating API Key needs Cookie auth
	// client.Jar should capture cookies automatically from Login response

	// 3. Create API Key
	apiKeyPayload := map[string]string{"name": "test-key-go"}
	keyBody, _ := json.Marshal(apiKeyPayload)
	resp, err = client.Post(baseURL+"/api-keys", "application/json", bytes.NewBuffer(keyBody))
	if err != nil {
		t.Fatalf("Create API Key request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Create API Key failed: %d", resp.StatusCode)
	}
	var keyResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&keyResp); err != nil {
		t.Fatalf("Failed to decode key response: %v", err)
	}
	apiKey := keyResp["key"]
	if apiKey == "" {
		t.Fatal("Empty API Key returned")
	}
	t.Logf("Generated API Key: %s", apiKey)

	// 4. Verify API Key Usage (Create Group)
	// Create a NEW client to ensure NO COOKIES are used, proving API Key works
	apiClient := &http.Client{}

	groupPayload := map[string]string{"name": "Go Test Group"}
	groupBody, _ := json.Marshal(groupPayload)
	req, _ := http.NewRequest("POST", baseURL+"/groups", bytes.NewBuffer(groupBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = apiClient.Do(req)
	if err != nil {
		t.Fatalf("Create Group req failed: %v", err)
	}
	if resp.StatusCode != 201 {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		t.Fatalf("Create Group failed: %d Body: %s", resp.StatusCode, buf.String())
	}
	var groupResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&groupResp); err != nil {
		t.Fatalf("Failed to decode group response: %v", err)
	}
	groupID := groupResp["id"]
	if groupID == "" {
		t.Fatal("Empty Group ID")
	}

	// 5. Verify API Key Usage (Create Monitor)
	monPayload := map[string]interface{}{
		"name":     "Go Monitor",
		"url":      "https://example.com",
		"groupId":  groupID,
		"interval": 60,
	}
	monBody, _ := json.Marshal(monPayload)
	req, _ = http.NewRequest("POST", baseURL+"/monitors", bytes.NewBuffer(monBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = apiClient.Do(req)
	if err != nil {
		t.Fatalf("Create Monitor req failed: %v", err)
	}
	if resp.StatusCode != 201 {
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(resp.Body); err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		t.Fatalf("Create Monitor failed: %d Body: %s", resp.StatusCode, buf.String())
	}

	// Check if monitor is in DB
	checkMon, _ := store.GetMonitors()
	found := false
	for _, m := range checkMon {
		if strings.Contains(m.Name, "Go Monitor") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Monitor not found in DB after API creation")
	}

	t.Log("Success: API Key Integration Test Passed")
}
