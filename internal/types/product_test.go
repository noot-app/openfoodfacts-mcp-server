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
