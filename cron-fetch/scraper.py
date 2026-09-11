"""Site checking logic.

For each site we pull individual job listings, read the experience requirement,
location, and role from each listing's text, and record only the new ones that
match the site's `min_experience`, `locations`, and `roles` filters (all must
pass). Listings that fail any one of them are skipped.
"""
import datetime as dt
import hashlib
import json
from urllib.parse import urljoin

import requests
from bs4 import BeautifulSoup

from database import get_connection
from detect import detect_listings
from experience import experience_matches, parse_experience
from location import find_location
from role import find_role

USER_AGENT = "CronFetch/0.1 (+https://github.com/cronfetch)"
MAX_TITLE_LEN = 500


def _utcnow():
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds")


def _log(conn, message, status):
    conn.execute(
        "INSERT INTO history (message, status) VALUES (?, ?)", (message, status)
    )


def _fetch_html(url):
    resp = requests.get(url, timeout=15, headers={"User-Agent": USER_AGENT})
    resp.raise_for_status()
    return resp.text


def _extract_listings(html, base_url, selector):
    """Return [(title, url), ...] for each element matched by `selector`."""
    soup = BeautifulSoup(html, "html.parser")
    listings = []
    for node in soup.select(selector) if selector else []:
        title = node.get_text(" ", strip=True)
        if not title:
            continue
        href = None
        if node.name == "a" and node.get("href"):
            href = node["href"]
        else:
            link = node.find("a", href=True)
            if link:
                href = link["href"]
        job_url = urljoin(base_url, href) if href else base_url
        listings.append((title[:MAX_TITLE_LEN], job_url))
    return listings


def _job_key(site_id, title, job_url):
    raw = f"{site_id}|{title}|{job_url}".encode("utf-8")
    return hashlib.sha256(raw).hexdigest()


def probe_site(url):
    """Fetch `url` and try to auto-detect the job-listing selector.

    Used by the API when a site is added without a selector. Never raises.
    """
    try:
        html = _fetch_html(url)
    except Exception as exc:
        return {
            "ok": False, "selector": None, "count": 0, "sample": [],
            "alternatives": [], "note": f"could not fetch page: {exc}",
        }
    found = detect_listings(html, url)
    return {
        "ok": bool(found["selector"]),
        "selector": found["selector"],
        "count": found["count"],
        "sample": [item["title"] for item in found["listings"][:5]],
        "alternatives": found.get("alternatives", []),
        "note": found["note"],
    }


def _resolve_selector(conn, site, html, now, result):
    """Return a usable selector for this site, auto-detecting and saving one if
    none is stored. Returns None (and records why) when detection fails."""
    selector = site["item_selector"] or site["css_selector"]
    if selector:
        return selector

    found = detect_listings(html, site["url"])
    if found["selector"]:
        conn.execute(
            "UPDATE sites SET item_selector = ?, selector_source = ? WHERE id = ?",
            (found["selector"], "auto", site["id"]),
        )
        _log(
            conn,
            f"Auto-detected listings for {site['name']}: "
            f"{found['selector']} ({found['count']} found)",
            "info",
        )
        conn.commit()
        return found["selector"]

    _log(
        conn,
        f"Could not find job listings for {site['name']}: {found['note']}",
        "error",
    )
    conn.execute("UPDATE sites SET last_checked = ? WHERE id = ?", (now, site["id"]))
    conn.commit()
    result["error"] = found["note"]
    return None


def check_site(conn, site):
    """Check one site. Returns a per-site summary dict."""
    now = _utcnow()
    result = {"site": site["name"], "new": 0, "skipped": 0, "seeded": 0, "error": None}
    first_run = site["last_checked"] is None

    try:
        html = _fetch_html(site["url"])
    except Exception as exc:
        _log(conn, f"Error checking {site['name']}: {exc}", "error")
        conn.execute("UPDATE sites SET last_checked = ? WHERE id = ?", (now, site["id"]))
        conn.commit()
        result["error"] = str(exc)
        return result

    selector = _resolve_selector(conn, site, html, now, result)
    if selector is None:
        return result

    locations = json.loads(site["locations"]) if site["locations"] else None
    roles = json.loads(site["roles"]) if site["roles"] else None
    page_hash = hashlib.sha256(html.encode("utf-8", "replace")).hexdigest()
    listings = _extract_listings(html, site["url"], selector)

    for title, job_url in listings:
        key = _job_key(site["id"], title, job_url)
        if conn.execute(
            "SELECT 1 FROM jobs WHERE site_id = ? AND job_key = ?", (site["id"], key)
        ).fetchone():
            continue  # already seen

        parsed = parse_experience(title)
        if site["min_experience"] is not None and not experience_matches(
            site["min_experience"], parsed
        ):
            result["skipped"] += 1
            continue

        matched_location = find_location(locations, title)
        if locations and not matched_location:
            result["skipped"] += 1
            continue

        matched_role = find_role(roles, title)
        if roles and not matched_role:
            result["skipped"] += 1
            continue

        emin, emax = parsed if parsed else (None, None)
        conn.execute(
            "INSERT INTO jobs (site_id, title, url, job_key, experience_min, experience_max, "
            "matched_location, matched_role) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
            (site["id"], title, job_url, key, emin, emax, matched_location, matched_role),
        )
        if first_run:
            result["seeded"] += 1  # existing listings recorded silently, no notification
        else:
            _log(conn, f"New match on {site['name']}: {title[:120]}", "pending")
            result["new"] += 1

    conn.execute(
        "UPDATE sites SET last_checked = ?, last_content_hash = ? WHERE id = ?",
        (now, page_hash, site["id"]),
    )
    if first_run and result["seeded"]:
        _log(
            conn,
            f"Seeded {result['seeded']} existing listing(s) for {site['name']} (no alerts sent)",
            "info",
        )
    conn.commit()
    return result


def run_check():
    """Check every site once."""
    conn = get_connection()
    try:
        sites = conn.execute("SELECT * FROM sites").fetchall()
        per_site = [check_site(conn, site) for site in sites]
        return {
            "sites": len(sites),
            "new_jobs": sum(s["new"] for s in per_site),
            "skipped": sum(s["skipped"] for s in per_site),
            "seeded": sum(s["seeded"] for s in per_site),
            "errors": sum(1 for s in per_site if s["error"]),
            "details": per_site,
        }
    finally:
        conn.close()
