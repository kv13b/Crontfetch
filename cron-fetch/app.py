"""CronFetch Flask application entrypoint."""
import atexit

from apscheduler.schedulers.background import BackgroundScheduler
from flask import Flask, jsonify
from flask_cors import CORS

from database import init_db
from routes import blueprints
from scraper import run_check

CHECK_INTERVAL_MINUTES = 15


def create_app():
    app = Flask(__name__)
    CORS(app)

    init_db()

    for bp in blueprints:
        app.register_blueprint(bp)

    @app.get("/health")
    def health():
        return jsonify({"status": "online"})

    return app


def start_scheduler():
    scheduler = BackgroundScheduler(daemon=True, timezone="UTC")
    scheduler.add_job(
        run_check,
        trigger="interval",
        minutes=CHECK_INTERVAL_MINUTES,
        id="scrape-check",
        max_instances=1,
        coalesce=True,
    )
    scheduler.start()
    atexit.register(lambda: scheduler.shutdown(wait=False))
    return scheduler


app = create_app()
scheduler = start_scheduler()


if __name__ == "__main__":
    # use_reloader=False so the background scheduler isn't started twice
    app.run(host="127.0.0.1", port=5000, debug=True, use_reloader=False)
