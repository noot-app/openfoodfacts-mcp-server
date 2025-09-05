package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
)

// Engine handles DuckDB queries against the parquet dataset
type Engine struct {
	db          *sql.DB
	parquetPath string
	log         *slog.Logger
}

// Ensure Engine implements QueryEngine interface
var _ QueryEngine = (*Engine)(nil)

// getDuckDBSettings returns DuckDB settings based on configuration
func getDuckDBSettings(cfg *config.Config, logger *slog.Logger) []string {
	logger.Info("DuckDB configuration",
		"memory_limit", cfg.DuckDBMemoryLimit,
		"threads", cfg.DuckDBThreads,
		"checkpoint_threshold", cfg.DuckDBCheckpointThreshold,
		"preserve_insertion_order", cfg.DuckDBPreserveInsertionOrder)

	settings := []string{
		// Core performance settings from config
		fmt.Sprintf("PRAGMA memory_limit='%s'", cfg.DuckDBMemoryLimit),
		fmt.Sprintf("PRAGMA threads=%d", cfg.DuckDBThreads),
		fmt.Sprintf("PRAGMA checkpoint_threshold='%s'", cfg.DuckDBCheckpointThreshold),

		// Performance optimizations from DuckDB guide
		"PRAGMA enable_progress_bar=false", // Disable progress bar for performance

		// Parquet-specific optimizations
		"PRAGMA enable_object_cache=true",        // Cache parquet metadata
		"PRAGMA enable_http_metadata_cache=true", // Cache remote file metadata
	}

	// Conditionally add preserve_insertion_order setting
	if !cfg.DuckDBPreserveInsertionOrder {
		settings = append(settings, "PRAGMA preserve_insertion_order=false")
	}

	return settings
}

// NewEngine creates a new query engine
func NewEngine(parquetPath string, cfg *config.Config, logger *slog.Logger) (*Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	// Configure connection pool for optimal performance
	// Based on DuckDB performance guide: fewer connections with more memory per connection
	db.SetMaxOpenConns(4)                   // Reduced to allow more memory per connection
	db.SetMaxIdleConns(2)                   // Keep fewer idle connections
	db.SetConnMaxLifetime(60 * time.Minute) // Longer lifetime for better cache reuse

	// Apply DuckDB performance optimizations based on configuration
	pragmaSettings := getDuckDBSettings(cfg, logger)

	for _, pragma := range pragmaSettings {
		if _, err := db.Exec(pragma); err != nil {
			logger.Warn("Failed to apply DuckDB optimization", "pragma", pragma, "error", err)
		}
	}

	engine := &Engine{
		db:          db,
		parquetPath: parquetPath,
		log:         logger,
	}

	return engine, nil
}

// Close closes the database connection
func (e *Engine) Close() error {
	return e.db.Close()
}

// convertPythonListToJSON converts Python-like list format to valid JSON
// OpenFoodFacts stores data in Python format with single quotes and NULL values
func convertPythonListToJSON(pythonStr string) string {
	// Replace single quotes with double quotes for JSON compatibility
	jsonStr := strings.ReplaceAll(pythonStr, "'", "\"")
	// Replace Python NULL with JSON null
	jsonStr = strings.ReplaceAll(jsonStr, "NULL", "null")

	// Handle unquoted string values - this is more complex and requires regex
	// For the specific case of unquoted values, we need to be careful not to quote numbers
	re := regexp.MustCompile(`"(\w+)":\s*([a-zA-Z][a-zA-Z0-9\-]*),`)
	jsonStr = re.ReplaceAllString(jsonStr, `"$1": "$2",`)

	// Handle unquoted strings at the end of objects (before })
	re2 := regexp.MustCompile(`"(\w+)":\s*([a-zA-Z][a-zA-Z0-9\-]*)\s*}`)
	jsonStr = re2.ReplaceAllString(jsonStr, `"$1": "$2"}`)

	return jsonStr
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SearchProductsByBrandAndName searches for products by name and brand
func (e *Engine) SearchProductsByBrandAndName(ctx context.Context, name, brand string, limit int) ([]Product, error) {
	totalStart := time.Now()
	e.log.Debug("SearchProductsByBrandAndName starting", "name", name, "brand", brand, "limit", limit)

	// Build optimized query with pre-computed text extraction
	// Performance guide optimization: avoid repeated complex expressions in WHERE clause
	var query string
	args := []interface{}{e.parquetPath}

	queryBuildStart := time.Now()

	if name != "" && brand != "" {
		// Most specific case - use CTE to pre-compute text fields once
		// Performance optimization: compute complex expressions only once
		query = `
		WITH extracted AS (
			SELECT 
				code,
				COALESCE(
					(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
					CAST(product_name AS VARCHAR)
				) as product_name_text,
				CAST(brands AS VARCHAR) as brands_text,
				CAST(nutriments AS VARCHAR) as nutriments_json,
				link,
				CAST(ingredients AS VARCHAR) as ingredients_json
			FROM read_parquet(?)
			-- Performance: filter on simpler field first (brands is typically smaller)
			WHERE brands IS NOT NULL 
			  AND CAST(brands AS VARCHAR) ILIKE ?
		)
		SELECT * FROM extracted 
		WHERE product_name_text IS NOT NULL 
		  AND product_name_text ILIKE ?
		ORDER BY length(product_name_text)  -- Performance: shorter names first for relevance
		LIMIT ?`

		brandPattern := fmt.Sprintf("%%%s%%", brand)
		namePattern := fmt.Sprintf("%%%s%%", name)
		args = append(args, brandPattern, namePattern, limit)

	} else if brand != "" {
		// Brand only - optimized for single filter
		query = `
		SELECT 
			code, 
			COALESCE(
				(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
				CAST(product_name AS VARCHAR)
			) as product_name_text,
			CAST(brands AS VARCHAR) as brands_text,
			CAST(nutriments AS VARCHAR) as nutriments_json,
			link,
			CAST(ingredients AS VARCHAR) as ingredients_json
		FROM read_parquet(?)
		WHERE brands IS NOT NULL 
		  AND CAST(brands AS VARCHAR) ILIKE ?
		ORDER BY code  -- Performance: leverage potential ordering on code
		LIMIT ?`

		brandPattern := fmt.Sprintf("%%%s%%", brand)
		args = append(args, brandPattern, limit)

	} else if name != "" {
		// Name only - avoid duplicate complex expression evaluation
		query = `
		WITH product_names AS (
			SELECT 
				code,
				COALESCE(
					(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
					CAST(product_name AS VARCHAR)
				) as product_name_text,
				CAST(brands AS VARCHAR) as brands_text,
				CAST(nutriments AS VARCHAR) as nutriments_json,
				link,
				CAST(ingredients AS VARCHAR) as ingredients_json
			FROM read_parquet(?)
			WHERE product_name IS NOT NULL
		)
		SELECT * FROM product_names
		WHERE product_name_text ILIKE ?
		ORDER BY length(product_name_text)  -- Performance: relevance ordering
		LIMIT ?`

		namePattern := fmt.Sprintf("%%%s%%", name)
		args = append(args, namePattern, limit)

	} else {
		// No filters - simple select with basic optimization
		query = `
		SELECT 
			code, 
			COALESCE(
				(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
				CAST(product_name AS VARCHAR)
			) as product_name_text,
			CAST(brands AS VARCHAR) as brands_text,
			CAST(nutriments AS VARCHAR) as nutriments_json,
			link,
			CAST(ingredients AS VARCHAR) as ingredients_json
		FROM read_parquet(?)
		WHERE product_name IS NOT NULL  -- Performance: filter out nulls early
		ORDER BY code  -- Performance: leverage potential ordering
		LIMIT ?`

		args = append(args, limit)
	}

	e.log.Debug("Query built", "duration", time.Since(queryBuildStart), "sql_length", len(query))

	queryStart := time.Now()
	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		e.log.Error("DuckDB query failed", "error", err, "duration", time.Since(queryStart))
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	e.log.Debug("Query executed", "duration", time.Since(queryStart))

	scanStart := time.Now()
	rowCount := 0

	var results []Product
	for rows.Next() {
		rowCount++
		var p Product
		var nutrimentsStr sql.NullString
		var ingredientsStr sql.NullString
		var linkStr sql.NullString
		var codeStr sql.NullString
		var productNameStr sql.NullString
		var brandsStr sql.NullString

		if err := rows.Scan(&codeStr, &productNameStr, &brandsStr, &nutrimentsStr, &linkStr, &ingredientsStr); err != nil {
			continue // Skip malformed rows
		}

		// Handle nullable fields
		if codeStr.Valid {
			p.Code = codeStr.String
		}
		if productNameStr.Valid {
			p.ProductName = productNameStr.String
		}
		if brandsStr.Valid {
			p.Brands = brandsStr.String
		}
		if linkStr.Valid {
			p.Link = linkStr.String
		}

		// Parse JSON fields
		if nutrimentsStr.Valid && nutrimentsStr.String != "" {
			// Convert Python-like format to valid JSON
			jsonStr := convertPythonListToJSON(nutrimentsStr.String)

			// Parse as array first since OpenFoodFacts stores it as an array of nutrient objects
			var nutrimentsArray []map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &nutrimentsArray); err != nil {
				e.log.Debug("Failed to parse nutriments JSON array", "error", err, "raw", nutrimentsStr.String[:min(100, len(nutrimentsStr.String))])
				p.Nutriments = make(map[string]interface{}) // Use empty map on parse error
			} else {
				// Convert array to map keyed by nutrient name
				nutrientsMap := make(map[string]interface{})
				for _, nutrient := range nutrimentsArray {
					if name, ok := nutrient["name"].(string); ok {
						nutrientsMap[name] = nutrient
					}
				}
				p.Nutriments = nutrientsMap
			}
		} else {
			p.Nutriments = make(map[string]interface{})
		}
		if ingredientsStr.Valid && ingredientsStr.String != "" {
			var ingredients interface{}
			if err := json.Unmarshal([]byte(ingredientsStr.String), &ingredients); err != nil {
				p.Ingredients = ingredientsStr.String // Use raw string on parse error
			} else {
				p.Ingredients = ingredients
			}
		}

		results = append(results, p)
	}

	if err := rows.Err(); err != nil {
		e.log.Error("Rows iteration failed", "error", err)
		return nil, fmt.Errorf("rows error: %w", err)
	}

	e.log.Debug("Row scanning completed", "rows_scanned", rowCount, "scan_duration", time.Since(scanStart))

	totalDuration := time.Since(totalStart)
	e.log.Info("SearchProductsByBrandAndName completed", "count", len(results), "total_duration", totalDuration)
	return results, nil
}

// SearchByBarcode searches for a product by barcode (exact match)
func (e *Engine) SearchByBarcode(ctx context.Context, barcode string) (*Product, error) {
	start := time.Now()
	e.log.Debug("SearchByBarcode starting", "barcode", barcode)

	// Performance optimization: exact match on code should be very fast
	// Use simple query structure for best performance on parquet
	query := `
		SELECT 
			code, 
			COALESCE(
				(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
				CAST(product_name AS VARCHAR)
			) as product_name_text,
			CAST(brands AS VARCHAR) as brands_text,
			CAST(nutriments AS VARCHAR) as nutriments_json,
			link,
			CAST(ingredients AS VARCHAR) as ingredients_json
		FROM read_parquet(?)
		WHERE code = ?
		LIMIT 1`

	rows, err := e.db.QueryContext(ctx, query, e.parquetPath, barcode)
	if err != nil {
		return nil, fmt.Errorf("barcode query failed: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		e.log.Debug("No product found for barcode", "barcode", barcode, "duration", time.Since(start))
		return nil, nil
	}

	var p Product
	var nutrimentsStr sql.NullString
	var ingredientsStr sql.NullString
	var linkStr sql.NullString
	var codeStr sql.NullString
	var productNameStr sql.NullString
	var brandsStr sql.NullString

	if err := rows.Scan(&codeStr, &productNameStr, &brandsStr, &nutrimentsStr, &linkStr, &ingredientsStr); err != nil {
		e.log.Error("Row scan failed", "error", err)
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// Handle nullable fields
	if codeStr.Valid {
		p.Code = codeStr.String
	}
	if productNameStr.Valid {
		p.ProductName = productNameStr.String
	}
	if brandsStr.Valid {
		p.Brands = brandsStr.String
	}
	if linkStr.Valid {
		p.Link = linkStr.String
	}

	// Parse JSON fields
	if nutrimentsStr.Valid && nutrimentsStr.String != "" {
		// Convert Python-like format to valid JSON
		jsonStr := convertPythonListToJSON(nutrimentsStr.String)

		// Parse as array first since OpenFoodFacts stores it as an array of nutrient objects
		var nutrimentsArray []map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &nutrimentsArray); err != nil {
			e.log.Debug("Failed to parse nutriments JSON array", "error", err, "raw", nutrimentsStr.String[:min(100, len(nutrimentsStr.String))])
			p.Nutriments = make(map[string]interface{}) // Use empty map on parse error
		} else {
			// Convert array to map keyed by nutrient name
			nutrientsMap := make(map[string]interface{})
			for _, nutrient := range nutrimentsArray {
				if name, ok := nutrient["name"].(string); ok {
					nutrientsMap[name] = nutrient
				}
			}
			p.Nutriments = nutrientsMap
		}
	} else {
		p.Nutriments = make(map[string]interface{})
	}

	if ingredientsStr.Valid && ingredientsStr.String != "" {
		var ingredients interface{}
		if err := json.Unmarshal([]byte(ingredientsStr.String), &ingredients); err != nil {
			p.Ingredients = ingredientsStr.String // Use raw string on parse error
		} else {
			p.Ingredients = ingredients
		}
	}

	duration := time.Since(start)
	e.log.Info("SearchByBarcode completed", "found", true, "duration", duration)
	return &p, nil
}

// TestConnection tests the database connection and parquet file access
func (e *Engine) TestConnection(ctx context.Context) error {
	start := time.Now()
	e.log.Debug("Testing DuckDB connection and parquet file")

	// Test basic connectivity and get file stats
	query := `SELECT COUNT(*) FROM read_parquet(?)`
	var count int64
	err := e.db.QueryRowContext(ctx, query, e.parquetPath).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to test parquet file access: %w", err)
	}

	// Analyze parquet file structure for performance insights
	go func() {
		e.analyzeParquetStructure(context.Background())
	}()

	e.log.Info("Connection test successful", "total_records", count, "duration", time.Since(start))
	return nil
}

// analyzeParquetStructure analyzes the parquet file structure and provides performance insights
func (e *Engine) analyzeParquetStructure(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			e.log.Warn("Parquet analysis failed", "error", r)
		}
	}()

	// Get parquet file statistics using simpler approach
	statsQuery := `
	SELECT 
		COUNT(*) as total_rows,
		COUNT(DISTINCT code) as unique_products
	FROM read_parquet(?)`

	var totalRows, uniqueProducts int64
	err := e.db.QueryRowContext(ctx, statsQuery, e.parquetPath).Scan(&totalRows, &uniqueProducts)
	if err != nil {
		e.log.Warn("Could not get parquet statistics", "error", err)
		return
	}

	// Try to get parquet metadata if available
	metadataQuery := `SELECT * FROM parquet_schema(?) LIMIT 1`
	rows, err := e.db.QueryContext(ctx, metadataQuery, e.parquetPath)
	if err != nil {
		e.log.Warn("Could not analyze parquet schema", "error", err)
	} else {
		rows.Close()

		e.log.Info("Parquet file analysis complete",
			"total_rows", totalRows,
			"unique_products", uniqueProducts,
			"performance_insights", []string{
				"File loaded successfully with optimized DuckDB settings",
				fmt.Sprintf("Processing %d total rows with %d unique products", totalRows, uniqueProducts),
			})
	}
}
