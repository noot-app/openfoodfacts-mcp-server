package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// MCPRequest represents a JSON-RPC 2.0 request for MCP
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// MCPToolCallParams represents the parameters for a tool call
type MCPToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// MCPResponse represents a JSON-RPC 2.0 response from MCP
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// MCPToolResult represents the result of a tool call
type MCPToolResult struct {
	Content []map[string]interface{} `json:"content"`
	IsError bool                     `json:"isError"`
}

const (
	serverURL   = "http://localhost:8080/mcp"
	authToken   = "your-secret-token"
	maxDuration = 1 * time.Second
	testRuns    = 5
)

func main() {
	fmt.Printf("🧪 Running MCP acceptance tests for OpenFoodFacts MCP Server\n")
	fmt.Printf("Expected: All MCP tool calls should complete in under %v\n", maxDuration)
	fmt.Printf("Validating: Product codes, brands, names, and ingredient data via MCP protocol\n\n")

	// Test case: Complex query (name + brand) using MCP tool
	toolArgs := map[string]interface{}{
		"name":  "cream soda",
		"brand": "olipop",
		"limit": 10,
	}

	var totalDuration time.Duration
	var maxDur time.Duration
	var minDur = time.Hour // Initialize to a large value

	fmt.Printf("Running MCP tool call %d times: search_products_by_brand_and_name with args %v\n", testRuns, toolArgs)

	for i := 1; i <= testRuns; i++ {
		start := time.Now()

		// Make the MCP request
		products, err := makeMCPToolCall("search_products_by_brand_and_name", toolArgs, i)
		if err != nil {
			fmt.Printf("❌ Test %d failed: %v\n", i, err)
			os.Exit(1)
		}

		duration := time.Since(start)
		totalDuration += duration

		if duration > maxDur {
			maxDur = duration
		}
		if duration < minDur {
			minDur = duration
		}

		// Validate response
		if len(products) == 0 {
			fmt.Printf("❌ Test %d failed: No products found\n", i)
			os.Exit(1)
		}

		// Validate expected products and data integrity
		if err := validateExpectedData(products, i); err != nil {
			fmt.Printf("❌ Test %d failed: %v\n", i, err)
			os.Exit(1)
		}

		// Performance check
		status := "✅"
		if duration > maxDuration {
			status = "❌"
		}

		fmt.Printf("%s Test %d: %v (%d products found)\n", status, i, duration, len(products))

		// Fail fast if performance requirement not met
		if duration > maxDuration {
			fmt.Printf("\n❌ FAILED: MCP tool call took %v, which exceeds the %v limit\n", duration, maxDuration)
			os.Exit(1)
		}
	}

	// Calculate statistics
	avgDuration := totalDuration / testRuns

	fmt.Printf("\n📊 Performance Summary:\n")
	fmt.Printf("   Runs: %d\n", testRuns)
	fmt.Printf("   Average: %v\n", avgDuration)
	fmt.Printf("   Min: %v\n", minDur)
	fmt.Printf("   Max: %v\n", maxDur)
	fmt.Printf("   Limit: %v\n", maxDuration)

	if maxDur <= maxDuration {
		fmt.Printf("\n🎉 ALL MCP TESTS PASSED! All tool calls completed within the performance requirements.\n")
	} else {
		fmt.Printf("\n❌ MCP TESTS FAILED! Some tool calls exceeded the performance requirements.\n")
		os.Exit(1)
	}
}

func makeMCPToolCall(toolName string, args map[string]interface{}, requestID int) ([]map[string]interface{}, error) {
	// Create MCP JSON-RPC 2.0 request
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  "tools/call",
		Params: MCPToolCallParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	// Marshal the request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers for MCP
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("MCP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse MCP response
	var mcpResponse MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResponse); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	// Check for JSON-RPC error
	if mcpResponse.Error != nil {
		return nil, fmt.Errorf("MCP error: %v", mcpResponse.Error)
	}

	// Extract tool result
	resultData, err := json.Marshal(mcpResponse.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(resultData, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	if toolResult.IsError {
		return nil, fmt.Errorf("tool returned error")
	}

	if len(toolResult.Content) == 0 {
		return nil, fmt.Errorf("no content in tool result")
	}

	// Parse the JSON content from the first text content item
	firstContent := toolResult.Content[0]
	if text, ok := firstContent["text"].(string); ok {
		var products []map[string]interface{}
		if err := json.Unmarshal([]byte(text), &products); err != nil {
			return nil, fmt.Errorf("failed to parse product JSON: %w", err)
		}
		return products, nil
	}

	return nil, fmt.Errorf("unexpected content format in MCP response")
}

// validateExpectedData checks for specific expected data points in the response
func validateExpectedData(products []map[string]interface{}, testNum int) error {
	if len(products) == 0 {
		return fmt.Errorf("no products returned")
	}

	// Expected product codes that should be present
	expectedCodes := []string{
		"0850027702186", // Cream Soda with detailed ingredients
		"0850027702407", // Cream Soda
		"0850027702711", // Cream Soda (Ambient)
		"0850027702896", // Cream Soda (4 Pack)
		"0850062639102", // Cream Soda
	}

	// Track which expected codes we found
	foundCodes := make(map[string]bool)
	foundCreamSoda := false
	foundOlipopBrand := false
	foundExpectedIngredients := false

	for _, product := range products {
		// Check product code
		if code, ok := product["code"].(string); ok {
			for _, expectedCode := range expectedCodes {
				if code == expectedCode {
					foundCodes[code] = true
				}
			}
		}

		// Check product name
		if productName, ok := product["product_name"].(string); ok {
			if productName == "Cream Soda" || productName == "Cream Soda (Ambient)" || productName == "Cream Soda (4 Pack)" {
				foundCreamSoda = true
			}
		}

		// Check brand
		if brands, ok := product["brands"].(string); ok {
			if brands == "Olipop" || brands == "OLIPOP" {
				foundOlipopBrand = true
			}
		}

		// Check for expected ingredients in detailed products
		if ingredients, ok := product["ingredients"].([]interface{}); ok && len(ingredients) > 0 {
			if hasExpectedIngredients(ingredients) {
				foundExpectedIngredients = true
			}
		}
	}

	// Validate we found at least some expected data
	if !foundCreamSoda {
		return fmt.Errorf("no Cream Soda product found in results")
	}

	if !foundOlipopBrand {
		return fmt.Errorf("no Olipop brand found in results")
	}

	// We should find at least 2 of the expected product codes
	if len(foundCodes) < 2 {
		return fmt.Errorf("expected at least 2 known product codes, found %d: %v", len(foundCodes), getFoundCodesList(foundCodes))
	}

	// At least one product should have detailed ingredients
	if !foundExpectedIngredients {
		return fmt.Errorf("no products with expected ingredient details found")
	}

	return nil
}

// hasExpectedIngredients checks if the ingredients contain expected Olipop ingredients
func hasExpectedIngredients(ingredients []interface{}) bool {
	expectedIngredientIDs := []string{
		"en:carbonated-water",
		"en:cassava-root-fiber",
		"en:chicory-root-inulin",
		"en:stevia-leaf",
		"en:himalayan-pink-salt",
		"en:monk-fruit",
	}

	foundCount := 0
	for _, ingredient := range ingredients {
		if ing, ok := ingredient.(map[string]interface{}); ok {
			if id, ok := ing["id"].(string); ok {
				for _, expectedID := range expectedIngredientIDs {
					if id == expectedID {
						foundCount++
						break
					}
				}
			}
		}
	}

	// Should find at least 3 expected ingredients
	return foundCount >= 3
}

// getFoundCodesList returns a list of found codes for error reporting
func getFoundCodesList(foundCodes map[string]bool) []string {
	var codes []string
	for code := range foundCodes {
		codes = append(codes, code)
	}
	return codes
}
