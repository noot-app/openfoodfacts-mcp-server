package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
)

// Query engine constants
const (
	MaxJSONDebugLength = 100
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
	db.SetMaxOpenConns(cfg.DuckDBMaxOpenConns)                                    // Configurable max connections
	db.SetMaxIdleConns(cfg.DuckDBMaxIdleConns)                                    // Configurable idle connections
	db.SetConnMaxLifetime(time.Duration(cfg.DuckDBConnMaxLifetime) * time.Minute) // Configurable lifetime

	logger.Info("DuckDB connection pool configured",
		"max_open_conns", cfg.DuckDBMaxOpenConns,
		"max_idle_conns", cfg.DuckDBMaxIdleConns,
		"conn_max_lifetime_minutes", cfg.DuckDBConnMaxLifetime)

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

// queryWithRetry executes a query with retry logic to handle brief file unavailability
func (e *Engine) queryWithRetry(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		rows, err := e.db.QueryContext(ctx, query, args...)
		if err == nil {
			return rows, nil
		}

		// Check if this is a file access error that might be temporary
		if strings.Contains(err.Error(), "No such file") ||
			strings.Contains(err.Error(), "cannot open") ||
			strings.Contains(err.Error(), "file not found") {

			if attempt < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<attempt) // exponential backoff
				e.log.Debug("Query failed with file access error, retrying",
					"attempt", attempt+1, "delay", delay, "error", err)

				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		}

		return nil, err
	}

	return nil, fmt.Errorf("query failed after %d attempts", maxRetries)
}

// queryRowWithRetry executes a QueryRow with retry logic
func (e *Engine) queryRowWithRetry(ctx context.Context, query string, args ...interface{}) *sql.Row {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		row := e.db.QueryRowContext(ctx, query, args...)

		// For QueryRow, we can't easily detect file errors until we scan
		// So we'll attempt the scan and retry if needed
		var testScan interface{}
		err := row.Scan(&testScan)

		if err == nil || err == sql.ErrNoRows {
			// Success or expected no rows - return a fresh row for the caller
			return e.db.QueryRowContext(ctx, query, args...)
		}

		// Check if this is a file access error that might be temporary
		if strings.Contains(err.Error(), "No such file") ||
			strings.Contains(err.Error(), "cannot open") ||
			strings.Contains(err.Error(), "file not found") {

			if attempt < maxRetries-1 {
				delay := baseDelay * time.Duration(1<<attempt)
				e.log.Debug("QueryRow failed with file access error, retrying",
					"attempt", attempt+1, "delay", delay, "error", err)

				select {
				case <-ctx.Done():
					return row // Return the failed row, caller will handle context cancellation
				case <-time.After(delay):
					continue
				}
			}
		}

		return row // Return the failed row for the caller to handle
	}

	// This shouldn't be reached, but return a fresh attempt
	return e.db.QueryRowContext(ctx, query, args...)
}

// convertPythonListToJSON converts Python-like list format to valid JSON
// OpenFoodFacts stores data in Python format with single quotes and NULL values
func convertPythonListToJSON(pythonStr string) string {
	if pythonStr == "" {
		return "[]"
	}

	// First replace single quotes with double quotes
	jsonStr := strings.ReplaceAll(pythonStr, "'", `"`)

	// Replace None/NULL with null (case-insensitive) - must be done BEFORE quoting unquoted strings
	replacements := []struct{ old, new string }{
		{" None", " null"},
		{"[None", "[null"},
		{",None", ",null"},
		{"None]", "null]"},
		{"None,", "null,"},
		{": None", ": null"},
		{" NULL", " null"},
		{"[NULL", "[null"},
		{",NULL", ",null"},
		{"NULL]", "null]"},
		{"NULL,", "null,"},
		{": NULL", ": null"},
		{" none", " null"},
		{"[none", "[null"},
		{",none", ",null"},
		{"none]", "null]"},
		{"none,", "null,"},
		{": none", ": null"},
	}

	for _, r := range replacements {
		jsonStr = strings.ReplaceAll(jsonStr, r.old, r.new)
	}

	// Handle unquoted string values using regex
	// This will quote unquoted identifiers like: sodium, mg, g, kcal, % vol, etc.
	// Pattern matches: ": word" or ": word-with-dashes" or ": % vol"
	// BUT EXCLUDES numbers and already converted 'null' values
	re := regexp.MustCompile(`: ([a-zA-Z%][a-zA-Z0-9\-_%\s]*)([ ,}\]])`)
	jsonStr = re.ReplaceAllStringFunc(jsonStr, func(match string) string {
		// Extract the matched groups
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}

		value := submatches[1]
		suffix := submatches[2]

		// Don't quote 'null' values that we just converted
		if value == "null" {
			return ": " + value + suffix
		}

		// Quote everything else
		return `: "` + value + `"` + suffix
	})

	return jsonStr
}

// parseNutrimentsJSON parses nutriments JSON data with comprehensive error handling
func (e *Engine) parseNutrimentsJSON(nutrimentsStr sql.NullString) map[string]interface{} {
	if !nutrimentsStr.Valid || nutrimentsStr.String == "" {
		return make(map[string]interface{})
	}

	rawStr := nutrimentsStr.String

	// Convert Python-like format to valid JSON
	jsonStr := convertPythonListToJSON(rawStr)

	// Parse as array first since OpenFoodFacts stores it as an array of nutrient objects
	var nutrimentsArray []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &nutrimentsArray); err != nil {
		// Log the error with truncated raw data for debugging
		debugStr := rawStr
		if len(debugStr) > MaxJSONDebugLength {
			debugStr = debugStr[:MaxJSONDebugLength] + "..."
		}
		e.log.Debug("Failed to parse nutriments JSON array",
			"error", err,
			"raw", debugStr,
			"converted", jsonStr[:min(MaxJSONDebugLength, len(jsonStr))])
		return make(map[string]interface{}) // Use empty map on parse error
	}

	// Convert array to map keyed by nutrient name
	nutrientsMap := make(map[string]interface{})
	for _, nutrient := range nutrimentsArray {
		if name, ok := nutrient["name"].(string); ok && name != "" {
			nutrientsMap[name] = nutrient
		}
	}
	return nutrientsMap
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
				CAST(ingredients AS VARCHAR) as ingredients_json,
				serving_quantity,
				product_quantity_unit,
				serving_size
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
			CAST(ingredients AS VARCHAR) as ingredients_json,
			serving_quantity,
			product_quantity_unit,
			serving_size
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
				CAST(ingredients AS VARCHAR) as ingredients_json,
				serving_quantity,
				product_quantity_unit,
				serving_size
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
			CAST(ingredients AS VARCHAR) as ingredients_json,
			serving_quantity,
			product_quantity_unit,
			serving_size
		FROM read_parquet(?)
		WHERE product_name IS NOT NULL  -- Performance: filter out nulls early
		ORDER BY code  -- Performance: leverage potential ordering
		LIMIT ?`

		args = append(args, limit)
	}

	e.log.Debug("Query built", "duration", time.Since(queryBuildStart), "sql_length", len(query))

	queryStart := time.Now()
	rows, err := e.queryWithRetry(ctx, query, args...)
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
		var servingQuantity sql.NullString
		var productQuantityUnit sql.NullString
		var servingSize sql.NullString

		if err := rows.Scan(&codeStr, &productNameStr, &brandsStr, &nutrimentsStr, &linkStr, &ingredientsStr, &servingQuantity, &productQuantityUnit, &servingSize); err != nil {
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
		if productQuantityUnit.Valid {
			p.ServingQuantityUnit = productQuantityUnit.String
		}
		if servingSize.Valid {
			p.ServingSize = servingSize.String
		}

		// Handle serving_quantity which can be string, int, float, or null
		if servingQuantity.Valid && servingQuantity.String != "" {
			// Try to parse as JSON to handle various types
			var qty interface{}
			if err := json.Unmarshal([]byte(servingQuantity.String), &qty); err != nil {
				// If JSON parsing fails, use the raw string
				p.ServingQuantity = servingQuantity.String
			} else {
				p.ServingQuantity = qty
			}
		}

		// Parse JSON fields
		p.Nutriments = e.parseNutrimentsJSON(nutrimentsStr)
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
	e.log.Info("SearchProductsByBrandAndName completed", "count", len(results), "total_duration_ms", totalDuration.Milliseconds())
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
			CAST(ingredients AS VARCHAR) as ingredients_json,
			serving_quantity,
			product_quantity_unit,
			serving_size
		FROM read_parquet(?)
		WHERE code = ?
		LIMIT 1`

	rows, err := e.queryWithRetry(ctx, query, e.parquetPath, barcode)
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
	var servingQuantity sql.NullString
	var productQuantityUnit sql.NullString
	var servingSize sql.NullString

	if err := rows.Scan(&codeStr, &productNameStr, &brandsStr, &nutrimentsStr, &linkStr, &ingredientsStr, &servingQuantity, &productQuantityUnit, &servingSize); err != nil {
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
	if productQuantityUnit.Valid {
		p.ServingQuantityUnit = productQuantityUnit.String
	}
	if servingSize.Valid {
		p.ServingSize = servingSize.String
	}

	// Handle serving_quantity which can be string, int, float, or null
	if servingQuantity.Valid && servingQuantity.String != "" {
		// Try to parse as JSON to handle various types
		var qty interface{}
		if err := json.Unmarshal([]byte(servingQuantity.String), &qty); err != nil {
			// If JSON parsing fails, use the raw string
			p.ServingQuantity = servingQuantity.String
		} else {
			p.ServingQuantity = qty
		}
	}

	// Parse JSON fields
	p.Nutriments = e.parseNutrimentsJSON(nutrimentsStr)

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
	err := e.queryRowWithRetry(ctx, query, e.parquetPath).Scan(&count)
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
	// First check if file exists and is readable
	if _, err := os.Stat(e.parquetPath); err != nil {
		e.log.Debug("Parquet file not accessible for analysis", "path", e.parquetPath, "error", err)
		return
	}

	statsQuery := `
	SELECT 
		COUNT(*) as total_rows,
		COUNT(DISTINCT code) as unique_products
	FROM read_parquet(?)`

	var totalRows, uniqueProducts int64
	err := e.queryRowWithRetry(ctx, statsQuery, e.parquetPath).Scan(&totalRows, &uniqueProducts)
	if err != nil {
		// Check if it's an empty file or schema issue
		if strings.Contains(err.Error(), "no rows in result set") {
			e.log.Debug("Parquet file appears to be empty or has no data", "path", e.parquetPath)
		} else {
			e.log.Debug("Could not get parquet statistics", "error", err, "path", e.parquetPath)
		}
		return
	}

	// Try to get parquet metadata if available
	metadataQuery := `SELECT * FROM parquet_schema(?) LIMIT 1`
	rows, err := e.queryWithRetry(ctx, metadataQuery, e.parquetPath)
	if err != nil {
		e.log.Debug("Could not analyze parquet schema", "error", err)
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
