"""/api/notify — Telegram delivery status and manual controls."""
from flask import Blueprint, jsonify

import notifier
from database import get_connection

bp = Blueprint("notify", __name__)


@bp.get("/api/notify/status")
def notify_status():
    return jsonify({"configured": notifier.is_configured()})


@bp.post("/api/notify")
def notify_now():
    """Retry any match still stuck as 'pending' (e.g. Telegram was down)."""
    conn = get_connection()
    try:
        result = notifier.send_pending(conn)
        return jsonify(result)
    finally:
        conn.close()


@bp.post("/api/notify/test")
def notify_test():
    """Send a one-off message to confirm TELEGRAM_BOT_TOKEN/CHAT_ID work."""
    if not notifier.is_configured():
        return jsonify({
            "ok": False,
            "error": "Telegram not configured: set TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID (see .env.example)",
        }), 400

    ok, error = notifier.send_message("CronFetch: test message - your Telegram setup works.")
    if not ok:
        return jsonify({"ok": False, "error": error}), 502
    return jsonify({"ok": True})
