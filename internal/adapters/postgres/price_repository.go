package postgres

import (
	"database/sql"
	"fmt"
	"time"
)

// SavePrices saves price data to PostgreSQL
func SavePrices(db *sql.DB, prices []models.Price) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

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

	for _, p := range prices {
		_, err := stmt.Exec(p.Symbol, p.Price, time.Unix(0, p.Timestamp))
		if err != nil {
			return fmt.Errorf("failed to insert price for %s: %w", p.Symbol, err)
		}
	}

	return tx.Commit()
}
