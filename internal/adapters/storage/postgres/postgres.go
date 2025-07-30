package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"marketflow/config"
	"marketflow/internal/app/logger"
	"marketflow/internal/models/domain"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type PostgresRepository struct {
	db *sqlx.DB
}

func NewPostgresDB(cfg *config.Config) (*PostgresRepository, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Postgres.Host,
		cfg.Postgres.Port,
		cfg.Postgres.User,
		cfg.Postgres.Password,
		cfg.Postgres.DBName,
		cfg.Postgres.SSLMode,
	)

	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("failed to ping postgres", "error", err)
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	logger.Info("postgres connection established")
	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Close() error {
	logger.Info("closing postgres connection")
	return r.db.Close()
}

func (r *PostgresRepository) StoreStats(stat domain.PriceStats) error {
	ctx := context.Background()
	query := `
		INSERT INTO price_stats (pair_name, exchange, timestamp, average_price, min_price, max_price)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (pair_name, exchange, timestamp) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, stat.Pair, stat.Exchange, stat.Timestamp, stat.Average, stat.Min, stat.Max)
	if err != nil {
		logger.Error("failed to store stats", "pair", stat.Pair, "exchange", stat.Exchange, "error", err)
		return fmt.Errorf("failed to store stats: %w", err)
	}

	logger.Info("stored stats", "pair", stat.Pair, "exchange", stat.Exchange, "timestamp", stat.Timestamp)
	return nil
}

func (r *PostgresRepository) StoreStatsBatch(stats []domain.PriceStats) error {
	if len(stats) == 0 {
		return nil
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", "error", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO price_stats (pair_name, exchange, timestamp, average_price, min_price, max_price)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (pair_name, exchange, timestamp) DO NOTHING
	`)
	if err != nil {
		logger.Error("failed to prepare statement", "error", err)
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, stat := range stats {
		_, err := stmt.ExecContext(ctx, stat.Pair, stat.Exchange, stat.Timestamp,
			stat.Average, stat.Min, stat.Max)
		if err != nil {
			logger.Error("failed to execute batch insert", "index", i,
				"pair", stat.Pair, "exchange", stat.Exchange, "error", err)
			return fmt.Errorf("failed to execute batch insert at index %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info("stored batch stats", "count", len(stats))
	return nil
}

func (r *PostgresRepository) StoreLargeStatsBatch(stats []domain.PriceStats) error {
	if len(stats) == 0 {
		return nil
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", "error", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	valueStrings := make([]string, 0, len(stats))
	valueArgs := make([]interface{}, 0, len(stats)*6)
	for i, stat := range stats {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)",
			i*6+1, i*6+2, i*6+3, i*6+4, i*6+5, i*6+6))
		valueArgs = append(valueArgs, stat.Pair, stat.Exchange, stat.Timestamp,
			stat.Average, stat.Min, stat.Max)
	}

	query := fmt.Sprintf(`
		INSERT INTO price_stats (pair_name, exchange, timestamp, average_price, min_price, max_price)
		VALUES %s
		ON CONFLICT (pair_name, exchange, timestamp) DO NOTHING
	`, strings.Join(valueStrings, ","))

	_, err = tx.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		logger.Error("failed to execute bulk insert", "error", err)
		return fmt.Errorf("failed to execute bulk insert: %w", err)
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info("stored batch stats", "count", len(stats))
	return nil
}

func (r *PostgresRepository) GetStats(pair, exchange string, since time.Time) ([]domain.PriceStats, error) {
	ctx := context.Background()
	query := `
		SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
		FROM price_stats
		WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3
		ORDER BY timestamp ASC
	`
	rows, err := r.db.QueryContext(ctx, query, pair, exchange, since)
	if err != nil {
		logger.Error("failed to get stats", "pair", pair, "exchange", exchange, "error", err)
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer rows.Close()

	var stats []domain.PriceStats
	for rows.Next() {
		var s domain.PriceStats
		if err := rows.Scan(&s.Pair, &s.Exchange, &s.Timestamp, &s.Average, &s.Min, &s.Max); err != nil {
			logger.Error("failed to scan stats", "error", err)
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		logger.Error("rows error", "error", err)
		return nil, fmt.Errorf("rows error: %w", err)
	}

	logger.Info("retrieved stats", "pair", pair, "exchange", exchange, "count", len(stats))
	return stats, nil
}

func (r *PostgresRepository) GetLatest(ctx context.Context, exchange, pair string) (domain.PriceStats, error) {
	query := `
		SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
		FROM price_stats
		WHERE pair_name = $1 AND exchange = $2
		ORDER BY timestamp DESC
		LIMIT 1
	`
	var stats domain.PriceStats
	err := r.db.QueryRowContext(ctx, query, pair, exchange).Scan(
		&stats.Pair, &stats.Exchange, &stats.Timestamp, &stats.Average, &stats.Min, &stats.Max,
	)
	if err == sql.ErrNoRows {
		logger.Warn("no latest price found", "pair", pair, "exchange", exchange)
		return domain.PriceStats{}, fmt.Errorf("no latest price for %s:%s", exchange, pair)
	}
	if err != nil {
		logger.Error("failed to get latest price", "pair", pair, "exchange", exchange, "error", err)
		return domain.PriceStats{}, fmt.Errorf("failed to get latest price: %w", err)
	}

	logger.Info("got latest price", "pair", pair, "exchange", exchange, "price", stats.Average)
	return stats, nil
}

func (r *PostgresRepository) GetByPeriod(ctx context.Context, exchange, pair string, period time.Duration) ([]domain.PriceStats, error) {
	query := `
		SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
		FROM price_stats
		WHERE pair_name = $1 AND exchange = $2 AND timestamp >= NOW() - $3::interval
		ORDER BY timestamp ASC
	`
	rows, err := r.db.QueryContext(ctx, query, pair, exchange, fmt.Sprintf("%d seconds", int64(period/time.Second)))
	if err != nil {
		logger.Error("failed to get stats by period", "pair", pair, "exchange", exchange, "period", period, "error", err)
		return nil, fmt.Errorf("failed to get stats by period: %w", err)
	}
	defer rows.Close()

	var stats []domain.PriceStats
	for rows.Next() {
		var s domain.PriceStats
		if err := rows.Scan(&s.Pair, &s.Exchange, &s.Timestamp, &s.Average, &s.Min, &s.Max); err != nil {
			logger.Error("failed to scan stats", "error", err)
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		logger.Error("rows error", "error", err)
		return nil, fmt.Errorf("rows error: %w", err)
	}

	logger.Info("retrieved stats by period", "pair", pair, "exchange", exchange, "period", period, "count", len(stats))
	return stats, nil
}
