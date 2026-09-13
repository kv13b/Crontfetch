"""Sends new-job notifications to Telegram.

Reads TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID from the environment (see
.env.example) at call time, not at import time, so a .env loaded after this
module is imported still takes effect.

send_pending(conn) walks `history` rows with status='pending' that are linked
to a job (history.job_id), sends one Telegram message per row, and flips each
to 'sent' on success. On failure the row is left 'pending' so the next check
retries it automatically - nothing is lost, it just waits for Telegram (or the
network) to recover.

When no token/chat id is configured, send_pending() is a no-op: matches still
get recorded and logged, they just won't be pushed anywhere until you set the
two environment variables.
"""
import os

import requests

API_BASE = "https://api.telegram.org/bot{token}/sendMessage"
TIMEOUT = 10


def _config():
    return os.environ.get("TELEGRAM_BOT_TOKEN"), os.environ.get("TELEGRAM_CHAT_ID")


def is_configured():
    token, chat_id = _config()
    return bool(token and chat_id)


def send_message(text):
    """Send one plain-text message. Returns (ok: bool, error: str | None)."""
    token, chat_id = _config()
    if not token or not chat_id:
        return False, "Telegram not configured (set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID)"

    try:
        resp = requests.post(
            API_BASE.format(token=token),
            json={"chat_id": chat_id, "text": text, "disable_web_page_preview": False},
            timeout=TIMEOUT,
        )
        data = resp.json()
    except Exception as exc:
        return False, f"request failed: {exc}"

    if not data.get("ok"):
        return False, data.get("description", "unknown Telegram API error")
    return True, None


def format_job_message(row):
    """Build a plain-text message from a joined history+jobs+sites row."""
    lines = [f"New job match — {row['site_name'] or 'Unknown site'}", row["title"] or "(untitled)"]

    details = []
    if row["experience_min"] is not None or row["experience_max"] is not None:
        lo = row["experience_min"]
        hi = row["experience_max"]
        if hi is None:
            details.append(f"{lo:g}+ yrs")
        elif lo == hi:
            details.append(f"{lo:g} yrs")
        else:
            details.append(f"{lo:g}-{hi:g} yrs")
    if row["matched_location"]:
        details.append(row["matched_location"])
    if row["matched_role"]:
        details.append(row["matched_role"])
    if details:
        lines.append(" | ".join(details))

    if row["url"]:
        lines.append(row["url"])

    return "\n".join(lines)


def send_pending(conn):
    """Send every unsent match, marking each 'sent' on success.

    Uses the connection passed in (caller commits/closes it) so this can run
    inside the same transaction as run_check() without opening a second one.
    """
    if not is_configured():
        return {"sent": 0, "failed": 0, "skipped": True, "note": "Telegram not configured"}

    rows = conn.execute(
        """
        SELECT history.id AS history_id, jobs.title, jobs.url,
               jobs.experience_min, jobs.experience_max,
               jobs.matched_location, jobs.matched_role,
               sites.name AS site_name
        FROM history
        JOIN jobs ON jobs.id = history.job_id
        LEFT JOIN sites ON sites.id = jobs.site_id
        WHERE history.status = 'pending'
        ORDER BY history.id
        """
    ).fetchall()

    sent, failed = 0, 0
    for row in rows:
        ok, error = send_message(format_job_message(row))
        if ok:
            conn.execute("UPDATE history SET status = 'sent' WHERE id = ?", (row["history_id"],))
            sent += 1
        else:
            # Leave it 'pending' - the next run_check() will retry automatically.
            failed += 1
        conn.commit()

    return {"sent": sent, "failed": failed, "skipped": False}


if __name__ == "__main__":
    if not is_configured():
        print("Not configured: set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID (see .env.example)")
    else:
        ok, error = send_message("CronFetch: this is a test message from notifier.py")
        print("sent OK" if ok else f"failed: {error}")
