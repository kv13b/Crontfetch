"""/api/history — notification / check log."""
from flask import Blueprint, jsonify

from database import get_connection

bp = Blueprint("history", __name__)


@bp.get("/api/history")
def list_history():
    conn = get_connection()
    try:
        rows = conn.execute(
            "SELECT * FROM history ORDER BY datetime(sent_at) DESC, id DESC LIMIT 200"
        ).fetchall()
        return jsonify([dict(r) for r in rows])
    finally:
        conn.close()
