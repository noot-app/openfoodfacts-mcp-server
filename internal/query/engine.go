package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
)

// Engine handles DuckDB queries against the parquet dataset
type Engine struct {
	db          *sql.DB
	parquetPath string
	log         *slog.Logger
}

// Ensure Engine implements QueryEngine interface
var _ QueryEngine = (*Engine)(nil)

// NewEngine creates a new query engine
func NewEngine(parquetPath string, logger *slog.Logger) (*Engine, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	// Configure connection pool for optimal performance
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Apply DuckDB performance optimizations
	pragmaSettings := []string{
		"PRAGMA memory_limit='2GB'",
		"PRAGMA threads=4",
		"PRAGMA enable_progress_bar=false",
		"PRAGMA checkpoint_threshold='1GB'",
	}

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

// SearchProductsByBrandAndName searches for products by name and brand
func (e *Engine) SearchProductsByBrandAndName(ctx context.Context, name, brand string, limit int) ([]Product, error) {
	totalStart := time.Now()
	e.log.Debug("SearchProductsByBrandAndName starting", "name", name, "brand", brand, "limit", limit)

	// Build optimized query with pre-computed text extraction
	// Use simpler approach to avoid nested operations in WHERE clause
	var query string
	args := []interface{}{e.parquetPath}

	queryBuildStart := time.Now()

	if name != "" && brand != "" {
		// Most specific case - use CTE to pre-compute text fields once
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
			WHERE CAST(brands AS VARCHAR) ILIKE ?
		)
		SELECT * FROM extracted 
		WHERE product_name_text ILIKE ?
		LIMIT ?`

		brandPattern := fmt.Sprintf("%%%s%%", brand)
		namePattern := fmt.Sprintf("%%%s%%", name)
		args = append(args, brandPattern, namePattern, limit)

	} else if brand != "" {
		// Brand only - much simpler and faster
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
		WHERE CAST(brands AS VARCHAR) ILIKE ?
		LIMIT ?`

		brandPattern := fmt.Sprintf("%%%s%%", brand)
		args = append(args, brandPattern, limit)

	} else if name != "" {
		// Name only
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
		WHERE COALESCE(
			(SELECT list_extract(list_filter(product_name, x -> x.lang = 'en'), 1).text),
			CAST(product_name AS VARCHAR)
		) ILIKE ?
		LIMIT ?`

		namePattern := fmt.Sprintf("%%%s%%", name)
		args = append(args, namePattern, limit)

	} else {
		// No filters - simple select
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
			var nutriments map[string]interface{}
			if err := json.Unmarshal([]byte(nutrimentsStr.String), &nutriments); err != nil {
				p.Nutriments = make(map[string]interface{}) // Use empty map on parse error
			} else {
				p.Nutriments = nutriments
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
		var nutriments map[string]interface{}
		if err := json.Unmarshal([]byte(nutrimentsStr.String), &nutriments); err != nil {
			p.Nutriments = make(map[string]interface{}) // Use empty map on parse error
		} else {
			p.Nutriments = nutriments
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

	query := `SELECT COUNT(*) FROM read_parquet(?)`
	var count int64

	if err := e.db.QueryRowContext(ctx, query, e.parquetPath).Scan(&count); err != nil {
		e.log.Error("Connection test failed", "error", err, "duration", time.Since(start))
		return fmt.Errorf("connection test failed: %w", err)
	}

	e.log.Info("Connection test successful", "total_records", count, "duration", time.Since(start))
	return nil
}
