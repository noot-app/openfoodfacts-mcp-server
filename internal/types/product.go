package types

// Product represents a product from the Open Food Facts dataset
// This is the canonical Product struct used throughout the application
type Product struct {
	Code                string                 `json:"code"`
	ProductName         string                 `json:"product_name"`
	Brands              string                 `json:"brands"`
	Nutriments          map[string]interface{} `json:"nutriments"`
	Link                string                 `json:"link"`
	Ingredients         interface{}            `json:"ingredients"`
	ServingQuantity     interface{}            `json:"serving_quantity,omitempty"`
	ServingQuantityUnit string                 `json:"serving_quantity_unit,omitempty"`
	ServingSize         string                 `json:"serving_size,omitempty"`
}

// Nutriment represents nutritional information for a product
type Nutriment struct {
	Per100g         *float64 `json:"100g"`
	Name            string   `json:"name"`
	PreparedPer100g *float64 `json:"prepared_100g"`
	PreparedServing *float64 `json:"prepared_serving"`
	PreparedUnit    *string  `json:"prepared_unit"`
	PreparedValue   *float64 `json:"prepared_value"`
	Serving         *float64 `json:"serving"`
	Unit            *string  `json:"unit"`
	Value           *float64 `json:"value"`
}

// Ingredient represents an ingredient in a product
type Ingredient struct {
	CiqualFoodCode      *string                  `json:"ciqual_food_code"`
	CiqualProxyFoodCode *string                  `json:"ciqual_proxy_food_code"`
	EcobalyseCode       *string                  `json:"ecobalyse_code"`
	EcobalyseProxyCode  *string                  `json:"ecobalyse_proxy_code"`
	FromPalmOil         *string                  `json:"from_palm_oil"`
	ID                  string                   `json:"id"`
	Ingredients         []map[string]interface{} `json:"ingredients"`
	IsInTaxonomy        *int                     `json:"is_in_taxonomy"`
	Labels              *string                  `json:"labels"`
	Origins             *string                  `json:"origins"`
	Percent             *float64                 `json:"percent"`
	PercentEstimate     *float64                 `json:"percent_estimate"`
	PercentMax          *float64                 `json:"percent_max"`
	PercentMin          *float64                 `json:"percent_min"`
	Processing          *string                  `json:"processing"`
	Quantity            *string                  `json:"quantity"`
	QuantityG           *float64                 `json:"quantity_g"`
	Text                string                   `json:"text"`
	Vegan               *string                  `json:"vegan"`
	Vegetarian          *string                  `json:"vegetarian"`
}

// SimplifiedIngredient represents a lean ingredient structure for reduced token consumption
type SimplifiedIngredient struct {
	ID              string   `json:"id"`
	Text            string   `json:"text"`
	PercentEstimate *float64 `json:"percent_estimate"`
}

// SimplifiedProduct represents a lean product structure for reduced token consumption
type SimplifiedProduct struct {
	ProductName string                 `json:"product_name"`
	Brands      string                 `json:"brands"`
	Link        string                 `json:"link"`
	Nutriments  map[string]interface{} `json:"nutriments"`
	Ingredients []SimplifiedIngredient `json:"ingredients"`
}

// ToSimplified converts a full Product to a SimplifiedProduct
func (p *Product) ToSimplified() SimplifiedProduct {
	simplified := SimplifiedProduct{
		ProductName: p.ProductName,
		Brands:      p.Brands,
		Link:        p.Link,
		Nutriments:  p.Nutriments,
		Ingredients: []SimplifiedIngredient{},
	}

	// Convert ingredients if they exist
	if p.Ingredients != nil {
		if ingredientSlice, ok := p.Ingredients.([]interface{}); ok {
			for _, ingredientData := range ingredientSlice {
				if ingredientMap, ok := ingredientData.(map[string]interface{}); ok {
					ingredient := SimplifiedIngredient{}

					// Extract required fields
					if id, exists := ingredientMap["id"]; exists {
						if idStr, ok := id.(string); ok {
							ingredient.ID = idStr
						}
					}

					if text, exists := ingredientMap["text"]; exists {
						if textStr, ok := text.(string); ok {
							ingredient.Text = textStr
						}
					}

					if percentEst, exists := ingredientMap["percent_estimate"]; exists {
						if percentFloat, ok := percentEst.(float64); ok {
							ingredient.PercentEstimate = &percentFloat
						}
					}

					// Only add ingredient if it has required fields
					if ingredient.ID != "" && ingredient.Text != "" {
						simplified.Ingredients = append(simplified.Ingredients, ingredient)
					}
				}
			}
		}
	}

	return simplified
}
