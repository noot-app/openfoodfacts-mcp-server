package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	serverURL = "http://localhost:8080"
	authToken = "your-secret-token"
)

type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]string      `json:"clientInfo"`
}

func main() {
	fmt.Printf("🧪 Simple MCP API Key Authentication Test\n")
	fmt.Printf("Testing: API key authentication and basic MCP protocol\n\n")

	// Test 1: Health check (no auth required)
	fmt.Printf("1. Testing health endpoint (no auth)...\n")
	if err := testHealth(); err != nil {
		fmt.Printf("❌ Health check failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Health check passed\n\n")

	// Test 2: MCP endpoint without auth (should fail)
	fmt.Printf("2. Testing MCP endpoint without auth (should fail)...\n")
	if err := testMCPWithoutAuth(); err == nil {
		fmt.Printf("❌ MCP endpoint allowed access without auth!\n")
		os.Exit(1)
	}
	fmt.Printf("✅ MCP endpoint correctly rejected unauthenticated request\n\n")

	// Test 3: MCP endpoint with wrong auth (should fail)
	fmt.Printf("3. Testing MCP endpoint with wrong auth (should fail)...\n")
	if err := testMCPWithWrongAuth(); err == nil {
		fmt.Printf("❌ MCP endpoint allowed access with wrong auth!\n")
		os.Exit(1)
	}
	fmt.Printf("✅ MCP endpoint correctly rejected wrong API key\n\n")

	// Test 4: MCP endpoint with correct auth (should succeed)
	fmt.Printf("4. Testing MCP endpoint with correct auth...\n")
	if err := testMCPWithCorrectAuth(); err != nil {
		fmt.Printf("❌ MCP endpoint failed with correct auth: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ MCP endpoint accepted correct API key\n\n")

	fmt.Printf("🎉 All API key authentication tests passed!\n")
}

func testHealth() error {
	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	return nil
}

func testMCPWithoutAuth() error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("correctly rejected") // This is expected
	}

	return nil // This means it didn't reject (bad)
}

func testMCPWithWrongAuth() error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer wrong-api-key")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("correctly rejected") // This is expected
	}

	return nil // This means it didn't reject (bad)
}

func testMCPWithCorrectAuth() error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Check that we get a proper MCP initialize response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// MCP responses come as Server-Sent Events
	if !strings.Contains(string(body), "serverInfo") {
		return fmt.Errorf("response doesn't contain expected MCP initialize result")
	}

	return nil
}
