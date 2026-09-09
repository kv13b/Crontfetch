"""/api/sites — manage the list of watched sites."""
from flask import Blueprint, jsonify, request

from database import get_connection

bp = Blueprint("sites", __name__)


@bp.get("/api/sites")
def list_sites():
    conn = get_connection()
    try:
        rows = conn.execute(
            "SELECT * FROM sites ORDER BY datetime(created_at) DESC, id DESC"
        ).fetchall()
        return jsonify([dict(r) for r in rows])
    finally:
        conn.close()


@bp.post("/api/sites")
def create_site():
    data = request.get_json(silent=True) or {}
    name = (data.get("name") or "").strip()
    url = (data.get("url") or "").strip()
    css_selector = (data.get("css_selector") or "").strip() or None

    if not name or not url:
        return jsonify({"error": "name and url are required"}), 400

    conn = get_connection()
    try:
        cur = conn.execute(
            "INSERT INTO sites (name, url, css_selector) VALUES (?, ?, ?)",
            (name, url, css_selector),
        )
        conn.commit()
        row = conn.execute(
            "SELECT * FROM sites WHERE id = ?", (cur.lastrowid,)
        ).fetchone()
        return jsonify(dict(row)), 201
    finally:
        conn.close()


@bp.delete("/api/sites/<int:site_id>")
def delete_site(site_id):
    conn = get_connection()
    try:
        cur = conn.execute("DELETE FROM sites WHERE id = ?", (site_id,))
        conn.commit()
        if cur.rowcount == 0:
            return jsonify({"error": "site not found"}), 404
        return jsonify({"deleted": site_id})
    finally:
        conn.close()
