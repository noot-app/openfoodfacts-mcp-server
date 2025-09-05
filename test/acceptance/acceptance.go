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

type CallToolParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type SearchProductArgs struct {
	Name  string `json:"name"`
	Brand string `json:"brand"`
	Limit int    `json:"limit,omitempty"`
}

type InitializedParams struct{}

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

	// Test 5: MCP tool call for product search
	fmt.Printf("5. Testing MCP tool call for product search...\n")
	if err := testMCPToolCall(); err != nil {
		fmt.Printf("❌ MCP tool call failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ MCP tool call succeeded with valid results\n\n")

	fmt.Printf("🎉 All API key authentication tests passed!\n")
	fmt.Printf("💡 Your MCP server is production-ready with simple API key authentication.\n")
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
			ProtocolVersion: "2025-06-18",
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
			ProtocolVersion: "2025-06-18",
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
			ProtocolVersion: "2025-06-18",
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
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
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

func testMCPToolCall() error {
	// Step 1: Initialize the MCP session
	fmt.Printf("   📋 Initializing MCP session...\n")

	sessionID, err := initializeMCPSession()
	if err != nil {
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	fmt.Printf("   📋 MCP session initialized with ID: %s\n", sessionID)

	// Step 2: Run the query 5 times to test robustness
	fmt.Printf("   🔍 Running tests: 5 queries for Olipop Cream Soda...\n")

	for i := 1; i <= 5; i++ {
		fmt.Printf("   � Query %d/5: ", i)

		start := time.Now()

		// Make the tool call
		err := performSingleToolCall(sessionID, i+2) // ID starts from 3 (initialize was 1, initialized has no ID)
		if err != nil {
			return fmt.Errorf("query %d failed: %w", i, err)
		}

		duration := time.Since(start)

		// Verify response time is under 1 second
		if duration > time.Second {
			return fmt.Errorf("query %d took %v, expected under 1 second", i, duration)
		}

		fmt.Printf("✅ (%.3fs)\n", duration.Seconds())
	}

	fmt.Printf("   🎯 All 5 queries completed successfully with validation\n")
	return nil
}

func performSingleToolCall(sessionID string, requestID int) error {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  "tools/call",
		Params: CallToolParams{
			Name: "search_products_by_brand_and_name",
			Arguments: SearchProductArgs{
				Name:  "Cream Soda",
				Brand: "Olipop",
				Limit: 10,
			},
		},
	}

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	client := &http.Client{Timeout: 2 * time.Second} // Shorter timeout for robustness test
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse SSE format - extract JSON from data: lines
	responseStr := string(body)
	actualJSON := ""
	lines := strings.Split(responseStr, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			actualJSON += strings.TrimPrefix(line, "data: ")
		}
	}

	if actualJSON == "" {
		return fmt.Errorf("no data found in SSE response")
	}

	// Parse the MCP response to extract the tool result
	var mcpResponse map[string]interface{}
	if err := json.Unmarshal([]byte(actualJSON), &mcpResponse); err != nil {
		return fmt.Errorf("failed to parse MCP response JSON: %w", err)
	}

	// Extract the tool result text from result.content[0].text
	result, ok := mcpResponse["result"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("MCP response missing result field")
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return fmt.Errorf("MCP response missing content array")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("MCP response content[0] is not an object")
	}

	toolResultText, ok := firstContent["text"].(string)
	if !ok {
		return fmt.Errorf("MCP response content[0] missing text field")
	}

	// Parse the tool result JSON to validate specific product details
	var toolResult map[string]interface{}
	if err := json.Unmarshal([]byte(toolResultText), &toolResult); err != nil {
		return fmt.Errorf("failed to parse tool result JSON: %w", err)
	}

	// Verify found is true
	found, ok := toolResult["found"].(bool)
	if !ok || !found {
		return fmt.Errorf("expected found=true, got found=%v", toolResult["found"])
	}

	// Get products array
	products, ok := toolResult["products"].([]interface{})
	if !ok || len(products) == 0 {
		return fmt.Errorf("expected non-empty products array")
	}

	// Look for the specific Olipop Cream Soda product (code: 0850027702186)
	var targetProduct map[string]interface{}
	for _, prod := range products {
		product, ok := prod.(map[string]interface{})
		if !ok {
			continue
		}

		code, ok := product["code"].(string)
		if ok && code == "0850027702186" {
			targetProduct = product
			break
		}
	}

	if targetProduct == nil {
		return fmt.Errorf("expected product with code '0850027702186' not found in results")
	}

	// Validate specific fields for the target product
	productName, ok := targetProduct["product_name"].(string)
	if !ok || productName != "Cream Soda" {
		return fmt.Errorf("expected product_name='Cream Soda', got '%v'", targetProduct["product_name"])
	}

	brands, ok := targetProduct["brands"].(string)
	if !ok || brands != "Olipop" {
		return fmt.Errorf("expected brands='Olipop', got '%v'", targetProduct["brands"])
	}

	link, ok := targetProduct["link"].(string)
	if !ok || link == "" {
		return fmt.Errorf("expected non-empty link field, got '%v'", targetProduct["link"])
	}

	// Verify link is a reasonable URL (basic check)
	if !strings.HasPrefix(link, "http") {
		return fmt.Errorf("expected link to be a valid URL, got '%s'", link)
	}

	return nil
}

func initializeMCPSession() (string, error) {
	// Step 1: Send initialize request
	initReq := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2025-06-18",
			Capabilities:    map[string]interface{}{},
			ClientInfo: map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, _ := json.Marshal(initReq)
	httpReq, _ := http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("initialize request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("initialize failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Extract session ID from response headers
	sessionID := resp.Header.Get("Mcp-Session-Id")

	// Read and validate initialize response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read initialize response: %w", err)
	}

	if !strings.Contains(string(body), "serverInfo") {
		return "", fmt.Errorf("initialize response doesn't contain expected serverInfo")
	}

	// Step 2: Send initialized notification
	initializedReq := struct {
		JSONRPC string            `json:"jsonrpc"`
		Method  string            `json:"method"`
		Params  InitializedParams `json:"params"`
	}{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  InitializedParams{},
	}

	jsonData, _ = json.Marshal(initializedReq)
	httpReq, _ = http.NewRequest("POST", serverURL+"/mcp", bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err = client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("initialized notification failed: %w", err)
	}
	defer resp.Body.Close()

	// Accept both 200 and 202 for notifications
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("initialized notification failed with status %d: %s", resp.StatusCode, string(body))
	}

	return sessionID, nil
}
