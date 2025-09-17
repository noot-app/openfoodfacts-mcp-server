package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProduct_JSONSerialization(t *testing.T) {
	product := Product{
		Code:                "12345",
		ProductName:         "Test Product",
		Brands:              "Test Brand",
		Nutriments:          map[string]interface{}{"energy": 100, "fat": 5.5},
		Link:                "https://example.com",
		Ingredients:         map[string]interface{}{"text": "test ingredients"},
		ServingQuantity:     355,
		ServingQuantityUnit: "ml",
		ServingSize:         "1 can (355 ml)",
	}

	jsonData, err := json.Marshal(product)
	require.NoError(t, err)

	var unmarshaled Product
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, product.Code, unmarshaled.Code)
	assert.Equal(t, product.ProductName, unmarshaled.ProductName)
	assert.Equal(t, product.Brands, unmarshaled.Brands)
	assert.Equal(t, product.Link, unmarshaled.Link)
	assert.Equal(t, product.ServingQuantityUnit, unmarshaled.ServingQuantityUnit)
	assert.Equal(t, product.ServingSize, unmarshaled.ServingSize)

	// Check serving_quantity (should be preserved as float64 after JSON marshal/unmarshal)
	assert.Equal(t, float64(355), unmarshaled.ServingQuantity)
}

func TestProduct_ServingAttributesFlexibility(t *testing.T) {
	tests := []struct {
		name            string
		servingQuantity interface{}
		expected        interface{}
	}{
		{
			name:            "integer serving quantity",
			servingQuantity: 355,
			expected:        float64(355), // JSON numbers become float64
		},
		{
			name:            "float serving quantity",
			servingQuantity: 355.5,
			expected:        355.5,
		},
		{
			name:            "string serving quantity",
			servingQuantity: "355 ml",
			expected:        "355 ml",
		},
		{
			name:            "nil serving quantity",
			servingQuantity: nil,
			expected:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := Product{
				Code:            "12345",
				ProductName:     "Test Product",
				Brands:          "Test Brand",
				ServingQuantity: tt.servingQuantity,
			}

			jsonData, err := json.Marshal(product)
			require.NoError(t, err)

			var unmarshaled Product
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, unmarshaled.ServingQuantity)
		})
	}
}

func TestNutriment_JSONSerialization(t *testing.T) {
	nutriment := Nutriment{
		Name:            "energy",
		Per100g:         &[]float64{2255}[0],
		Serving:         &[]float64{564}[0],
		Unit:            &[]string{"kJ"}[0],
		Value:           &[]float64{2255}[0],
		PreparedPer100g: &[]float64{2000}[0],
		PreparedServing: &[]float64{500}[0],
		PreparedUnit:    &[]string{"kJ"}[0],
		PreparedValue:   &[]float64{2000}[0],
	}

	jsonData, err := json.Marshal(nutriment)
	require.NoError(t, err)

	var unmarshaled Nutriment
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, nutriment.Name, unmarshaled.Name)
	assert.Equal(t, *nutriment.Per100g, *unmarshaled.Per100g)
	assert.Equal(t, *nutriment.Serving, *unmarshaled.Serving)
	assert.Equal(t, *nutriment.Unit, *unmarshaled.Unit)
	assert.Equal(t, *nutriment.Value, *unmarshaled.Value)
}

func TestIngredient_JSONSerialization(t *testing.T) {
	ingredient := Ingredient{
		ID:                  "sugar",
		Text:                "Sugar",
		Percent:             &[]float64{56.3}[0],
		PercentEstimate:     &[]float64{55.0}[0],
		PercentMin:          &[]float64{50.0}[0],
		PercentMax:          &[]float64{60.0}[0],
		Vegan:               &[]string{"yes"}[0],
		Vegetarian:          &[]string{"yes"}[0],
		FromPalmOil:         &[]string{"no"}[0],
		Processing:          &[]string{"refined"}[0],
		Quantity:            &[]string{"56.3g"}[0],
		QuantityG:           &[]float64{56.3}[0],
		CiqualFoodCode:      &[]string{"31016"}[0],
		CiqualProxyFoodCode: &[]string{"31016"}[0],
		EcobalyseCode:       &[]string{"sugar"}[0],
		EcobalyseProxyCode:  &[]string{"sugar"}[0],
		IsInTaxonomy:        &[]int{1}[0],
		Labels:              &[]string{"organic"}[0],
		Origins:             &[]string{"france"}[0],
		Ingredients:         []map[string]interface{}{{"id": "sugar", "text": "Sugar"}},
	}

	jsonData, err := json.Marshal(ingredient)
	require.NoError(t, err)

	var unmarshaled Ingredient
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, ingredient.ID, unmarshaled.ID)
	assert.Equal(t, ingredient.Text, unmarshaled.Text)
	assert.Equal(t, *ingredient.Percent, *unmarshaled.Percent)
	assert.Equal(t, *ingredient.Vegan, *unmarshaled.Vegan)
	assert.Equal(t, *ingredient.Vegetarian, *unmarshaled.Vegetarian)
	assert.Equal(t, len(ingredient.Ingredients), len(unmarshaled.Ingredients))
}

func TestProduct_ToSimplified(t *testing.T) {
	tests := []struct {
		name     string
		product  Product
		expected SimplifiedProduct
	}{
		{
			name: "basic product conversion",
			product: Product{
				Code:        "12345",
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com/product/12345",
				Nutriments: map[string]interface{}{
					"fat": map[string]interface{}{
						"100g":    5.5,
						"serving": 2.0,
						"unit":    "g",
					},
				},
				Ingredients: []interface{}{
					map[string]interface{}{
						"id":               "sugar",
						"text":             "Sugar",
						"percent_estimate": 25.5,
					},
					map[string]interface{}{
						"id":               "salt",
						"text":             "Salt",
						"percent_estimate": 2.0,
					},
				},
			},
			expected: SimplifiedProduct{
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com/product/12345",
				Nutriments: map[string]interface{}{
					"fat": map[string]interface{}{
						"100g":    5.5,
						"serving": 2.0,
						"unit":    "g",
					},
				},
				Ingredients: []SimplifiedIngredient{
					{
						ID:              "sugar",
						Text:            "Sugar",
						PercentEstimate: &[]float64{25.5}[0],
					},
					{
						ID:              "salt",
						Text:            "Salt",
						PercentEstimate: &[]float64{2.0}[0],
					},
				},
			},
		},
		{
			name: "product with incomplete ingredients",
			product: Product{
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com",
				Ingredients: []interface{}{
					map[string]interface{}{
						"id":   "sugar",
						"text": "Sugar",
					},
					map[string]interface{}{
						"id": "salt", // missing text - should be excluded
					},
					map[string]interface{}{
						"text": "Unknown", // missing id - should be excluded
					},
				},
			},
			expected: SimplifiedProduct{
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com",
				Ingredients: []SimplifiedIngredient{
					{
						ID:   "sugar",
						Text: "Sugar",
					},
				},
			},
		},
		{
			name: "product with nil ingredients",
			product: Product{
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com",
				Ingredients: nil,
			},
			expected: SimplifiedProduct{
				ProductName: "Test Product",
				Brands:      "Test Brand",
				Link:        "https://example.com",
				Ingredients: []SimplifiedIngredient{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.product.ToSimplified()
			assert.Equal(t, tt.expected.ProductName, result.ProductName)
			assert.Equal(t, tt.expected.Brands, result.Brands)
			assert.Equal(t, tt.expected.Link, result.Link)
			assert.Equal(t, tt.expected.Nutriments, result.Nutriments)
			assert.Equal(t, len(tt.expected.Ingredients), len(result.Ingredients))

			for i, expectedIngredient := range tt.expected.Ingredients {
				assert.Equal(t, expectedIngredient.ID, result.Ingredients[i].ID)
				assert.Equal(t, expectedIngredient.Text, result.Ingredients[i].Text)
				if expectedIngredient.PercentEstimate == nil {
					assert.Nil(t, result.Ingredients[i].PercentEstimate)
				} else {
					require.NotNil(t, result.Ingredients[i].PercentEstimate)
					assert.Equal(t, *expectedIngredient.PercentEstimate, *result.Ingredients[i].PercentEstimate)
				}
			}
		})
	}
}

func TestProduct_processNutrimentsForSimplified_EnergyRedaction(t *testing.T) {
	tests := []struct {
		name               string
		inputNutriments    map[string]interface{}
		expectedNutriments map[string]interface{}
		description        string
	}{
		{
			name: "keep energy-kcal and remove energy (kJ)",
			inputNutriments: map[string]interface{}{
				"energy": map[string]interface{}{
					"100g":    2255.0,
					"serving": 564.0,
					"unit":    "kJ",
					"name":    "energy",
				},
				"energy-kcal": map[string]interface{}{
					"100g":    539.0,
					"serving": 135.0,
					"unit":    "kcal",
					"name":    "energy-kcal",
				},
				"fat": map[string]interface{}{
					"100g": 5.5,
					"unit": "g",
				},
			},
			expectedNutriments: map[string]interface{}{
				"energy-kcal": map[string]interface{}{
					"100g":    539.0,
					"serving": 135.0,
					"unit":    "kcal",
					"name":    "energy-kcal",
				},
				"fat": map[string]interface{}{
					"100g": 5.5,
					"unit": "g",
				},
			},
			description: "When both energy and energy-kcal exist, keep only energy-kcal",
		},
		{
			name: "convert energy (kJ) to energy-kcal when energy-kcal missing",
			inputNutriments: map[string]interface{}{
				"energy": map[string]interface{}{
					"100g":    2255.0,
					"serving": 564.0,
					"value":   2255.0,
					"unit":    "kJ",
					"name":    "energy",
				},
				"protein": map[string]interface{}{
					"100g": 10.0,
					"unit": "g",
				},
			},
			expectedNutriments: map[string]interface{}{
				"energy-kcal": map[string]interface{}{
					"100g":    2255.0 / 4.184, // Direct calculation to match implementation
					"serving": 564.0 / 4.184,  // Direct calculation to match implementation
					"value":   2255.0 / 4.184, // Direct calculation to match implementation
					"unit":    "kcal",
					"name":    "energy-kcal",
				},
				"protein": map[string]interface{}{
					"100g": 10.0,
					"unit": "g",
				},
			},
			description: "When only energy (kJ) exists, convert to energy-kcal",
		},
		{
			name: "preserve nutriments without energy fields",
			inputNutriments: map[string]interface{}{
				"protein": map[string]interface{}{
					"100g": 10.0,
					"unit": "g",
				},
				"fat": map[string]interface{}{
					"100g": 5.5,
					"unit": "g",
				},
			},
			expectedNutriments: map[string]interface{}{
				"protein": map[string]interface{}{
					"100g": 10.0,
					"unit": "g",
				},
				"fat": map[string]interface{}{
					"100g": 5.5,
					"unit": "g",
				},
			},
			description: "When no energy fields exist, preserve all other nutriments",
		},
		{
			name:               "handle nil nutriments",
			inputNutriments:    nil,
			expectedNutriments: nil,
			description:        "When nutriments is nil, return nil",
		},
		{
			name:               "handle empty nutriments",
			inputNutriments:    map[string]interface{}{},
			expectedNutriments: map[string]interface{}{},
			description:        "When nutriments is empty, return empty map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := Product{
				Nutriments: tt.inputNutriments,
			}

			result := product.processNutrimentsForSimplified()

			if tt.expectedNutriments == nil {
				assert.Nil(t, result, tt.description)
			} else {
				require.NotNil(t, result, tt.description)
				assert.Equal(t, len(tt.expectedNutriments), len(result), tt.description)

				for key, expectedValue := range tt.expectedNutriments {
					actualValue, exists := result[key]
					assert.True(t, exists, "Expected key %s to exist in result", key)

					if expectedMap, ok := expectedValue.(map[string]interface{}); ok {
						actualMap, ok := actualValue.(map[string]interface{})
						require.True(t, ok, "Expected %s to be a map", key)

						for subKey, expectedSubValue := range expectedMap {
							actualSubValue, exists := actualMap[subKey]
							assert.True(t, exists, "Expected subkey %s.%s to exist", key, subKey)

							if expectedFloat, ok := expectedSubValue.(float64); ok {
								actualFloat, ok := actualSubValue.(float64)
								require.True(t, ok, "Expected %s.%s to be float64", key, subKey)
								assert.InDelta(t, expectedFloat, actualFloat, 0.1, "Values for %s.%s don't match", key, subKey)
							} else {
								assert.Equal(t, expectedSubValue, actualSubValue, "Values for %s.%s don't match", key, subKey)
							}
						}
					} else {
						assert.Equal(t, expectedValue, actualValue, "Values for %s don't match", key)
					}
				}

				// Ensure energy field is not present when it shouldn't be
				_, hasEnergy := result["energy"]
				assert.False(t, hasEnergy, "energy field should be removed")
			}
		})
	}
}
