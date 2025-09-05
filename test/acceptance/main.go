package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// QueryRequest represents the request payload for the query endpoint
type QueryRequest struct {
	Name  string `json:"name,omitempty"`
	Brand string `json:"brand,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// QueryResponse represents the response from the query endpoint
type QueryResponse struct {
	Found    bool                     `json:"found"`
	Products []map[string]interface{} `json:"products"`
}

const (
	serverURL   = "http://localhost:8080/query"
	authToken   = "your-secret-token"
	maxDuration = 1 * time.Second
	testRuns    = 5
)

func main() {
	fmt.Printf("🧪 Running acceptance tests for OpenFoodFacts MCP Server\n")
	fmt.Printf("Expected: All queries should complete in under %v\n\n", maxDuration)

	// Test case: Complex query (name + brand)
	req := QueryRequest{
		Name:  "cream soda",
		Brand: "olipop",
		Limit: 10,
	}

	var totalDuration time.Duration
	var maxDur time.Duration
	var minDur = time.Hour // Initialize to a large value

	fmt.Printf("Running query %d times: {\"name\": \"%s\", \"brand\": \"%s\"}\n", testRuns, req.Name, req.Brand)

	for i := 1; i <= testRuns; i++ {
		start := time.Now()

		// Make the HTTP request
		response, err := makeRequest(req)
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
		if !response.Found {
			fmt.Printf("❌ Test %d failed: No products found\n", i)
			os.Exit(1)
		}

		if len(response.Products) == 0 {
			fmt.Printf("❌ Test %d failed: Empty products array\n", i)
			os.Exit(1)
		}

		// Check for expected Olipop Cream Soda products
		foundCreamSoda := false
		for _, product := range response.Products {
			if productName, ok := product["product_name"].(string); ok {
				if productName == "Cream Soda" {
					foundCreamSoda = true
					break
				}
			}
		}

		if !foundCreamSoda {
			fmt.Printf("❌ Test %d failed: No Cream Soda product found in results\n", i)
			os.Exit(1)
		}

		// Performance check
		status := "✅"
		if duration > maxDuration {
			status = "❌"
		}

		fmt.Printf("%s Test %d: %v (%d products found)\n", status, i, duration, len(response.Products))

		// Fail fast if performance requirement not met
		if duration > maxDuration {
			fmt.Printf("\n❌ FAILED: Query took %v, which exceeds the %v limit\n", duration, maxDuration)
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
		fmt.Printf("\n🎉 ALL TESTS PASSED! All queries completed within the performance requirements.\n")
	} else {
		fmt.Printf("\n❌ TESTS FAILED! Some queries exceeded the performance requirements.\n")
		os.Exit(1)
	}
}

func makeRequest(req QueryRequest) (*QueryResponse, error) {
	// Marshal the request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+authToken)

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse response
	var response QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
