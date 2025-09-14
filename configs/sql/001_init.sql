-- users & transactions schema for auth + basic flow
CREATE TABLE IF NOT EXISTS users (
  id            UUID PRIMARY KEY,
  name          TEXT        NOT NULL,
  email         TEXT        NOT NULL UNIQUE,
  password_hash TEXT        NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transactions (
  transaction_id UUID PRIMARY KEY,
  user_id        UUID        NOT NULL REFERENCES users(id),
  amount         DOUBLE PRECISION NOT NULL,
  timestamp      TIMESTAMPTZ NOT NULL,
  status         TEXT        NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_timestamp ON transactions(timestamp);
