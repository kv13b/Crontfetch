"""/api/sites — manage the list of watched sites.

The user only has to supply `name` and `url`. When no `item_selector` is given,
the API fetches the page and auto-detects the job-listing selector itself (see
`detect.py`); the result is stored on the site and returned under `detection`.

`min_experience`, `locations`, and `roles` are independent filters (all must
pass for a listing to be recorded) - see `experience.py`, `location.py`, and
`role.py`.
"""
import json

from flask import Blueprint, jsonify, request

from database import get_connection
from location import parse_locations
from role import parse_roles
from scraper import probe_site

bp = Blueprint("sites", __name__)

INSERT_COLS = (
    "name", "url", "css_selector", "item_selector", "selector_source",
    "min_experience", "locations", "roles",
)

# List-style filters: API field -> parser that normalises raw input to a list or None.
LIST_FILTERS = {"locations": parse_locations, "roles": parse_roles}


def _clean_payload(data, *, require_core):
    """Normalise a site payload. Returns (values_dict, error_message)."""
    out = {}

    if "name" in data or require_core:
        out["name"] = (data.get("name") or "").strip()
    if "url" in data or require_core:
        out["url"] = (data.get("url") or "").strip()
    if "css_selector" in data:
        out["css_selector"] = (data.get("css_selector") or "").strip() or None
    if "item_selector" in data:
        out["item_selector"] = (data.get("item_selector") or "").strip() or None

    if "min_experience" in data:
        raw = data.get("min_experience")
        if raw in (None, ""):
            out["min_experience"] = None
        else:
            try:
                years = int(raw)
            except (TypeError, ValueError):
                return None, "min_experience must be a whole number of years or null"
            if years < 0:
                return None, "min_experience cannot be negative"
            out["min_experience"] = years

    for field, parser in LIST_FILTERS.items():
        if field in data:
            raw = data.get(field)
            if raw is not None and not isinstance(raw, (list, str)):
                return None, f"{field} must be a list of strings, a comma-separated string, or null"
            cleaned = parser(raw)
            out[field] = json.dumps(cleaned) if cleaned else None

    if require_core and (not out.get("name") or not out.get("url")):
        return None, "name and url are required"
    return out, None


def _get_site(conn, site_id):
    return conn.execute("SELECT * FROM sites WHERE id = ?", (site_id,)).fetchone()


def _serialize_site(row):
    """Row -> dict, decoding the list-filter JSON columns into plain lists."""
    site = dict(row)
    for field in LIST_FILTERS:
        site[field] = json.loads(site[field]) if site.get(field) else None
    return site


@bp.get("/api/sites")
def list_sites():
    conn = get_connection()
    try:
        rows = conn.execute(
            "SELECT * FROM sites ORDER BY datetime(created_at) DESC, id DESC"
        ).fetchall()
        return jsonify([_serialize_site(r) for r in rows])
    finally:
        conn.close()


@bp.post("/api/sites")
def create_site():
    data = request.get_json(silent=True) or {}
    values, error = _clean_payload(data, require_core=True)
    if error:
        return jsonify({"error": error}), 400

    manual_selector = values.get("item_selector")
    values["selector_source"] = "manual" if manual_selector else None

    conn = get_connection()
    try:
        cur = conn.execute(
            f"INSERT INTO sites ({', '.join(INSERT_COLS)}) "
            f"VALUES ({', '.join('?' * len(INSERT_COLS))})",
            [values.get(c) for c in INSERT_COLS],
        )
        conn.commit()
        site_id = cur.lastrowid

        detection = None
        if not manual_selector:
            detection = probe_site(values["url"])
            if detection["ok"]:
                conn.execute(
                    "UPDATE sites SET item_selector = ?, selector_source = 'auto' WHERE id = ?",
                    (detection["selector"], site_id),
                )
                conn.commit()

        body = _serialize_site(_get_site(conn, site_id))
        if detection is not None:
            body["detection"] = detection
        return jsonify(body), 201
    finally:
        conn.close()


@bp.patch("/api/sites/<int:site_id>")
def update_site(site_id):
    data = request.get_json(silent=True) or {}
    values, error = _clean_payload(data, require_core=False)
    if error:
        return jsonify({"error": error}), 400
    if not values:
        return jsonify({"error": "no editable fields supplied"}), 400

    if "item_selector" in values:
        values["selector_source"] = "manual" if values["item_selector"] else None

    assignments = ", ".join(f"{k} = ?" for k in values)
    conn = get_connection()
    try:
        cur = conn.execute(
            f"UPDATE sites SET {assignments} WHERE id = ?",
            [*values.values(), site_id],
        )
        conn.commit()
        if cur.rowcount == 0:
            return jsonify({"error": "site not found"}), 404
        return jsonify(_serialize_site(_get_site(conn, site_id)))
    finally:
        conn.close()


@bp.post("/api/sites/<int:site_id>/detect")
def redetect_site(site_id):
    """Re-run auto-detection for a site and store the result."""
    conn = get_connection()
    try:
        site = _get_site(conn, site_id)
        if site is None:
            return jsonify({"error": "site not found"}), 404

        detection = probe_site(site["url"])
        if detection["ok"]:
            conn.execute(
                "UPDATE sites SET item_selector = ?, selector_source = 'auto' WHERE id = ?",
                (detection["selector"], site_id),
            )
            conn.commit()

        body = _serialize_site(_get_site(conn, site_id))
        body["detection"] = detection
        return jsonify(body)
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
