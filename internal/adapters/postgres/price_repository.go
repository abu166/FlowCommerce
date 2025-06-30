package postgres

import (
	"database/sql"
	"fmt"
	"marketflow/internal/app/logger"
	"marketflow/internal/domain/models"
	"time"
)

// SavePrices saves price data to PostgreSQL
func SavePrices(db *sql.DB, prices []models.Price) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			logger.Warn("Rolling back transaction due to error", "error", err)
			tx.Rollback()
		}
	}()

	stmt, err := tx.Prepare(`
        INSERT INTO prices (symbol, price, timestamp)
        VALUES ($1, $2, $3)
        ON CONFLICT (symbol) DO UPDATE
        SET price = EXCLUDED.price, timestamp = EXCLUDED.timestamp
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, p := range prices {
		_, err := stmt.Exec(p.Symbol, p.Price, time.Unix(p.Timestamp, 0))
		if err != nil {
			return fmt.Errorf("failed to insert price for %s (index %d): %w", p.Symbol, i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
