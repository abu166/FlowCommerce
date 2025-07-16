package postgres

// SavePrices saves price data to PostgreSQL
// func SavePrices(db *sql.DB, prices []models.Price) error {
// 	tx, err := db.Begin()
// 	if err != nil {
// 		return fmt.Errorf("failed to begin transaction: %w", err)
// 	}
// 	defer func() {
// 		if err != nil {
// 			logger.Warn("Rolling back transaction due to error", "error", err)
// 			tx.Rollback()
// 		}
// 	}()

// 	// Изменили запрос - убрали ON CONFLICT
// 	stmt, err := tx.Prepare(`
//         INSERT INTO prices (symbol, price, timestamp)
//         VALUES ($1, $2, $3)

//     `)
// 	if err != nil {
// 		return fmt.Errorf("failed to prepare statement: %w", err)
// 	}
// 	defer stmt.Close()

// 	for i, p := range prices {
// 		// Используем оригинальный timestamp из данных
// 		_, err := stmt.Exec(p.Symbol, p.Price, time.Unix(p.Timestamp, 0))
// 		if err != nil {
// 			return fmt.Errorf("failed to insert price for %s (index %d): %w", p.Symbol, i, err)
// 		}
// 	}

// 	if err := tx.Commit(); err != nil {
// 		return fmt.Errorf("failed to commit transaction: %w", err)
// 	}

// 	logger.Info("Successfully saved prices", "count", len(prices))
// 	return nil
// }
