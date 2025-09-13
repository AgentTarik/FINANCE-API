-- Extension for gen_random_uuid() 
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id   UUID PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS transactions (
  transaction_id UUID PRIMARY KEY,
  user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  amount         DOUBLE PRECISION NOT NULL,
  timestamp      TIMESTAMPTZ NOT NULL,
  status         TEXT NOT NULL CHECK (status IN ('queued','processed','failed'))
);

CREATE INDEX IF NOT EXISTS idx_tx_user_ts ON transactions (user_id, timestamp DESC);
