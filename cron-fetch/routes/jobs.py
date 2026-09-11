"""/api/jobs — discovered jobs, plus the manual scraper trigger."""
from flask import Blueprint, jsonify

from database import get_connection

bp = Blueprint("jobs", __name__)


@bp.get("/api/jobs")
def list_jobs():
    conn = get_connection()
    try:
        rows = conn.execute(
            """
            SELECT jobs.id, jobs.site_id, jobs.title, jobs.url, jobs.found_at,
                   jobs.experience_min, jobs.experience_max,
                   jobs.matched_location, jobs.matched_role,
                   sites.name AS site_name
            FROM jobs
            LEFT JOIN sites ON sites.id = jobs.site_id
            ORDER BY datetime(jobs.found_at) DESC, jobs.id DESC
            LIMIT 100
            """
        ).fetchall()
        return jsonify([dict(r) for r in rows])
    finally:
        conn.close()


@bp.post("/api/run-now")
def run_now():
    from scraper import run_check

    result = run_check()
    return jsonify({"status": "ok", "result": result})
