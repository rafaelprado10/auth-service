-- Cria o banco auth_db se não existir
SELECT 'CREATE DATABASE auth_db'
WHERE NOT EXISTS (
    SELECT FROM pg_database WHERE datname = 'auth_db'
)\gexec

-- Conecta no banco auth_db
\connect auth_db

-- Cria a tabela api_keys se não existir
CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,

    -- hash SHA-256 (64 caracteres hex)
    key_hash VARCHAR(64) NOT NULL UNIQUE,

    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
