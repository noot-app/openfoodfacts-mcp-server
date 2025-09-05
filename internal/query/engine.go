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

	return &Engine{
		db:          db,
		parquetPath: parquetPath,
		log:         logger,
	}, nil
}

// Close closes the database connection
func (e *Engine) Close() error {
	return e.db.Close()
}

// SearchProducts searches for products by name and brand
func (e *Engine) SearchProducts(ctx context.Context, name, brand string, limit int) ([]Product, error) {
	start := time.Now()
	e.log.Debug("SearchProducts starting", "name", name, "brand", brand, "limit", limit)

	// Build the query with proper parameterization
	query := `
		SELECT code, product_name, brands, nutriments, link, ingredients
		FROM read_parquet(?)
		WHERE 1=1`

	args := []interface{}{e.parquetPath}

	if name != "" {
		query += ` AND product_name ILIKE ?`
		args = append(args, fmt.Sprintf("%%%s%%", name))
	}

	if brand != "" {
		query += ` AND brands ILIKE ?`
		args = append(args, fmt.Sprintf("%%%s%%", brand))
	}

	query += ` LIMIT ?`
	args = append(args, limit)

	rows, err := e.db.QueryContext(ctx, query, args...)
	if err != nil {
		e.log.Error("DuckDB query failed", "error", err, "duration", time.Since(start))
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var results []Product
	for rows.Next() {
		var p Product
		var nutrimentsStr sql.NullString
		var ingredientsStr sql.NullString
		var linkStr sql.NullString
		var codeStr sql.NullString
		var productNameStr sql.NullString
		var brandsStr sql.NullString

		if err := rows.Scan(&codeStr, &productNameStr, &brandsStr, &nutrimentsStr, &linkStr, &ingredientsStr); err != nil {
			e.log.Error("Row scan failed", "error", err)
			continue
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
				e.log.Debug("Failed to parse nutriments JSON", "error", err, "code", p.Code)
				p.Nutriments = make(map[string]interface{})
			} else {
				p.Nutriments = nutriments
			}
		} else {
			p.Nutriments = make(map[string]interface{})
		}

		if ingredientsStr.Valid && ingredientsStr.String != "" {
			var ingredients interface{}
			if err := json.Unmarshal([]byte(ingredientsStr.String), &ingredients); err != nil {
				e.log.Debug("Failed to parse ingredients JSON", "error", err, "code", p.Code)
				p.Ingredients = ingredientsStr.String
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

	duration := time.Since(start)
	e.log.Info("SearchProducts completed", "count", len(results), "duration", duration)
	return results, nil
}

// SearchByBarcode searches for a product by barcode (exact match)
func (e *Engine) SearchByBarcode(ctx context.Context, barcode string) (*Product, error) {
	start := time.Now()
	e.log.Debug("SearchByBarcode starting", "barcode", barcode)

	query := `
		SELECT code, product_name, brands, nutriments, link, ingredients
		FROM read_parquet(?)
		WHERE code = ?
		LIMIT 1`

	rows, err := e.db.QueryContext(ctx, query, e.parquetPath, barcode)
	if err != nil {
		e.log.Error("DuckDB barcode query failed", "error", err, "duration", time.Since(start))
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
			e.log.Debug("Failed to parse nutriments JSON", "error", err, "code", p.Code)
			p.Nutriments = make(map[string]interface{})
		} else {
			p.Nutriments = nutriments
		}
	} else {
		p.Nutriments = make(map[string]interface{})
	}

	if ingredientsStr.Valid && ingredientsStr.String != "" {
		var ingredients interface{}
		if err := json.Unmarshal([]byte(ingredientsStr.String), &ingredients); err != nil {
			e.log.Debug("Failed to parse ingredients JSON", "error", err, "code", p.Code)
			p.Ingredients = ingredientsStr.String
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
