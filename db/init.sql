

DROP TABLE IF EXISTS prices;

-- Создайте заново
CREATE TABLE price_aggregates (
    id SERIAL PRIMARY KEY,
    pair_name VARCHAR(50) NOT NULL,
    exchange VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    average_price DOUBLE PRECISION NOT NULL,
    min_price DOUBLE PRECISION NOT NULL,
    max_price DOUBLE PRECISION NOT NULL,
    UNIQUE (pair_name, exchange, timestamp)
);
CREATE INDEX idx_prices_symbol ON price_aggregates (symbol);
CREATE INDEX idx_prices_timestamp ON price_aggregates (timestamp);