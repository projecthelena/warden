package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

var baseURL string

func main() {
	flag.StringVar(&baseURL, "base-url", envOr("WARDEN_URL", "http://localhost:9096"), "Warden base URL")
	username := flag.String("username", envOr("WARDEN_USERNAME", "admin"), "Warden username")
	password := flag.String("password", os.Getenv("WARDEN_PASSWORD"), "Warden password (prefer WARDEN_PASSWORD)")
	targetURL := flag.String("target-url", envOr("TARGET_URL", "http://localhost:8888/healthy"), "URL monitored by generated monitors")
	count := flag.Int("count", 50, "number of monitors to create")
	interval := flag.Int("interval", 10, "check interval in seconds")
	deleteMonitors := flag.Bool("delete", false, "delete created monitors and their group after creation")
	flag.Parse()
	baseURL = strings.TrimRight(baseURL, "/")
	if *password == "" {
		log.Fatal("WARDEN_PASSWORD or -password is required")
	}
	if *count < 1 || *interval < 10 {
		log.Fatal("count must be positive and interval must be at least 10 seconds")
	}

	// 1. Setup Client with Cookie Jar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// 2. Login
	log.Println("Logging in...")
	if err := login(client, *username, *password); err != nil {
		log.Fatalf("Login failed: %v", err)
	}

	// 3. Create Group
	groupName := "Load Test " + time.Now().UTC().Format("20060102-150405")
	groupID, err := createGroup(client, groupName)
	if err != nil {
		log.Fatalf("Failed to create group: %v", err)
	}
	log.Printf("Created group %s\n", groupID)

	// 4. Create Monitors
	log.Printf("Creating %d monitors...\n", *count)
	var monitorIDs []string
	for i := 0; i < *count; i++ {
		name := fmt.Sprintf("Load Monitor %05d", i)

		id, err := createMonitor(client, name, *targetURL, groupID, *interval)
		if err != nil {
			log.Printf("Failed to create monitor %d: %v", i, err)
			continue
		}
		monitorIDs = append(monitorIDs, id)
		fmt.Printf(".")
		if (i+1)%10 == 0 {
			fmt.Println()
		}
	}
	fmt.Println("\nDone creating monitors.")
	log.Printf("group=%s monitors=%d target=%s interval=%ds", groupID, len(monitorIDs), *targetURL, *interval)

	if *deleteMonitors {
		log.Println("Deleting monitors...")
		for _, id := range monitorIDs {
			if err := deleteMonitor(client, id); err != nil {
				log.Printf("Failed to delete monitor %s: %v", id, err)
			}
		}
		log.Println("Deleting group...")
		if err := deleteGroup(client, groupID); err != nil {
			log.Printf("Failed to delete group: %v", err)
		}
		log.Println("Cleanup done.")
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func login(client *http.Client, username, password string) error {
	payload := map[string]string{"username": username, "password": password}
	data, _ := json.Marshal(payload)
	resp, err := client.Post(baseURL+"/api/auth/login", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func createGroup(client *http.Client, name string) (string, error) {
	payload := map[string]string{"name": name}
	data, _ := json.Marshal(payload)
	resp, err := client.Post(baseURL+"/api/groups", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	var res map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res["id"].(string), nil
}

func createMonitor(client *http.Client, name, url, groupID string, interval int) (string, error) {
	payload := map[string]interface{}{
		"name":     name,
		"url":      url,
		"groupId":  groupID,
		"interval": interval,
	}
	data, _ := json.Marshal(payload)
	resp, err := client.Post(baseURL+"/api/monitors", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var res map[string]interface{} // API currently returns the monitor object
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	// Depending on API response structure, ID might be direct or nested?
	// Based on handlers_monitors.go, it returns the monitor JSON.
	// Monitor struct has "id" json tag.
	if id, ok := res["id"].(string); ok {
		return id, nil
	}
	return "", fmt.Errorf("no id in response")
}

func deleteMonitor(client *http.Client, id string) error {
	req, _ := http.NewRequest("DELETE", baseURL+"/api/monitors/"+id, nil)
	resp, err := client.Do(req) // #nosec G704 -- baseURL is hardcoded localhost
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func deleteGroup(client *http.Client, id string) error {
	req, _ := http.NewRequest("DELETE", baseURL+"/api/groups/"+id, nil)
	resp, err := client.Do(req) // #nosec G704 -- baseURL is hardcoded localhost
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
