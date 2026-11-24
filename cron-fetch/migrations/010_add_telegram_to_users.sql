-- The Telegram chat alerts go to. TEXT because telegram.SendMessage takes the
-- chat ID as a string. NULL means the user hasn't connected Telegram yet.
ALTER TABLE users ADD COLUMN telegram_chat_id TEXT;

-- One-time codes for the "Connect Telegram" link (t.me/<bot>?start=<code>).
-- A code is deleted once used and ignored after expires_at.
CREATE TABLE IF NOT EXISTS telegram_link_codes (
    code TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
