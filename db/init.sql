
DROP TABLE IF EXISTS prices;

-- Создайте заново
CREATE TABLE prices (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    price DECIMAL(20,8) NOT NULL,
    timestamp TIMESTAMP NOT NULL,
     created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prices_symbol ON prices(symbol);
CREATE INDEX idx_prices_timestamp ON prices(timestamp);