"""Site checking logic: fetch each site, detect content changes, log results."""
import datetime as dt
import hashlib

import requests
from bs4 import BeautifulSoup

from database import get_connection

USER_AGENT = "CronFetch/0.1 (+https://github.com/cronfetch)"


def _utcnow():
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds")


def _fetch_content(url, css_selector):
    resp = requests.get(url, timeout=15, headers={"User-Agent": USER_AGENT})
    resp.raise_for_status()
    soup = BeautifulSoup(resp.text, "html.parser")
    if css_selector:
        nodes = soup.select(css_selector)
        return "\n".join(n.get_text(" ", strip=True) for n in nodes)
    return soup.get_text(" ", strip=True)


def run_check():
    """Check every site once. Records a job + history row when content changes."""
    conn = get_connection()
    checked = 0
    changed = 0
    try:
        sites = conn.execute("SELECT * FROM sites").fetchall()
        for site in sites:
            now = _utcnow()
            try:
                content = _fetch_content(site["url"], site["css_selector"])
            except Exception as exc:  # network / parse failure — log and move on
                conn.execute(
                    "INSERT INTO history (message, status) VALUES (?, ?)",
                    (f"Error checking {site['name']}: {exc}", "error"),
                )
                conn.execute(
                    "UPDATE sites SET last_checked = ? WHERE id = ?", (now, site["id"])
                )
                conn.commit()
                continue

            digest = hashlib.sha256(content.encode("utf-8")).hexdigest()
            checked += 1
            if site["last_content_hash"] and site["last_content_hash"] != digest:
                changed += 1
                conn.execute(
                    "INSERT INTO jobs (site_id, title, url) VALUES (?, ?, ?)",
                    (site["id"], f"Change detected on {site['name']}", site["url"]),
                )
                conn.execute(
                    "INSERT INTO history (message, status) VALUES (?, ?)",
                    (f"Change detected on {site['name']}", "pending"),
                )
            conn.execute(
                "UPDATE sites SET last_checked = ?, last_content_hash = ? WHERE id = ?",
                (now, digest, site["id"]),
            )
            conn.commit()
        return {"sites": len(sites), "checked": checked, "changed": changed}
    finally:
        conn.close()
