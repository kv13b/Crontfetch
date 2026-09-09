"""Blueprint registry for the CronFetch REST API."""
from routes.history import bp as history_bp
from routes.jobs import bp as jobs_bp
from routes.sites import bp as sites_bp

blueprints = (sites_bp, jobs_bp, history_bp)
