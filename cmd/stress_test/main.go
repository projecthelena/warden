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
	"time"
)

const baseURL = "http://localhost:9096" // Adjust port if running on different port

func main() {
	count := flag.Int("count", 50, "Number of monitors to create")
	delete := flag.Bool("delete", false, "Delete created monitors after wait")
	flag.Parse()

	// 1. Setup Client with Cookie Jar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	// 2. Login
	log.Println("Logging in...")
	if err := login(client, "admin", "password"); err != nil {
		// Try admin/password if admin/admin fails? No, "admin" usually has password "admin" in dev seed.
		// If fails, we can't proceed.
		log.Fatalf("Login failed: %v", err)
	}

	// 3. Create Group
	groupID, err := createGroup(client, "Stress Test Group")
	if err != nil {
		log.Fatalf("Failed to create group: %v", err)
	}
	log.Printf("Created group %s\n", groupID)

	// 4. Create Monitors
	log.Printf("Creating %d monitors...\n", *count)
	var monitorIDs []string
	for i := 0; i < *count; i++ {
		// Alternate between 200 and 500 to trigger notifications
		status := 200
		if i%2 == 0 {
			status = 500 // Will trigger DOWN
		}

		name := fmt.Sprintf("Stress Monitor %d (%d)", i, status)
		url := fmt.Sprintf("https://httpbin.org/status/%d", status)

		id, err := createMonitor(client, name, url, groupID)
		if err != nil {
			log.Printf("Failed to create monitor %d: %v", i, err)
			continue
		}
		monitorIDs = append(monitorIDs, id)
		fmt.Printf(".")
		if (i+1)%10 == 0 {
			fmt.Println()
		}
		// Small sleep to not overwhelm completely
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println("\nDone creating monitors.")

	if *delete {
		log.Println("Waiting 30 seconds before deletion...")
		time.Sleep(30 * time.Second)
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

func createMonitor(client *http.Client, name, url, groupID string) (string, error) {
	payload := map[string]interface{}{
		"name":     name,
		"url":      url,
		"groupId":  groupID,
		"interval": 60,
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
