-- Таблица коротких ссылок
CREATE TABLE IF NOT EXISTS urls (
    id         BIGINT PRIMARY KEY,
    short_url  VARCHAR(12) UNIQUE NOT NULL,
    long_url   TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индекс для поиска по оригинальному URL (дедупликация)
CREATE INDEX IF NOT EXISTS idx_urls_long_url ON urls USING hash(long_url);
