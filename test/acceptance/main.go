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
	fmt.Printf("Expected: All queries should complete in under %v\n", maxDuration)
	fmt.Printf("Validating: Product codes, brands, names, and ingredient data\n\n")

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

		// Validate expected products and data integrity
		if err := validateExpectedData(response.Products, i); err != nil {
			fmt.Printf("❌ Test %d failed: %v\n", i, err)
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
