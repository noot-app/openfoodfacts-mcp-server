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
