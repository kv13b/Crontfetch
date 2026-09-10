"""Automatic detection of the repeating 'job card' element on a listings page.

The user only pastes a URL. detect_listings() fetches nothing itself - it takes
already-downloaded HTML and returns the best CSS selector for one job listing,
plus the listings it found with that selector.

Strategy:
  1. If the host is a known ATS (Greenhouse, Lever, ...), try its known selector.
  2. Otherwise, group every link's ancestors by structural signature
     (tag + classes + parent), then score the groups that look like a list of
     jobs: many similar siblings, each with its own link, job-ish text/urls.
  3. If nothing scores and the page looks JS-rendered, say so.
"""
import warnings
from collections import Counter
from urllib.parse import urljoin, urlparse

from bs4 import BeautifulSoup

try:
    from bs4 import XMLParsedAsHTMLWarning

    warnings.filterwarnings("ignore", category=XMLParsedAsHTMLWarning)
except ImportError:  # older bs4
    pass

MAX_CLIMB = 6
MIN_ITEMS = 3
MIN_CONFIDENCE = 55       # below this we don't trust the auto-detected selector
MIN_TEXT_HINT = 0.25      # fraction of cards whose text contains a role word

JOB_HREF_HINTS = (
    "job", "career", "position", "opening", "vacancy", "requisition",
    "gh_jid", "posting", "listing", "/p/", "apply",
)
JOB_TEXT_HINTS = (
    "engineer", "manager", "analyst", "developer", "designer", "lead",
    "director", "specialist", "consultant", "intern", "coordinator", "architect",
    "scientist", "representative", "administrator", "associate", "officer",
    "recruiter", "counsel", "marketing", "sales", "remote", "hybrid",
)

# host substring -> selector to try first (None = known to be JS-rendered)
KNOWN_ATS = {
    "greenhouse.io": "div.opening, .job-post",
    "lever.co": "div.posting",
    "smartrecruiters.com": "li.opening-job",
    "workable.com": "li[data-ui='job']",
    "bamboohr.com": "li.BambooHR-ATS-Jobs-Item",
    "myworkdayjobs.com": None,
    "ashbyhq.com": None,
    "icims.com": None,
}

JS_MARKERS = (
    "__next_data__", "window.__nuxt__", "ng-version=", "data-reactroot",
    'id="__next"', 'id="root"', "data-server-rendered",
)


def _sig(node):
    classes = ".".join(sorted(node.get("class", [])))
    return f"{node.name}.{classes}" if classes else node.name


def _looks_js_rendered(html):
    low = html[:200000].lower()
    return any(m in low for m in JS_MARKERS)


def _abs(base_url, href):
    return urljoin(base_url, href.strip())


def _selector_for(parent, child_sig):
    """Build a CSS selector for a child with `child_sig` under `parent`."""
    tag, _, classes = child_sig.partition(".")
    if classes:
        return f"{tag}.{classes}"

    # no classes on the card - anchor it to the parent
    if parent is None:
        return tag
    if parent.get("id"):
        return f"#{parent['id']} > {tag}"
    parent_classes = ".".join(sorted(parent.get("class", [])))
    if parent_classes:
        return f"{parent.name}.{parent_classes} > {tag}"
    return f"{parent.name} > {tag}"


def _extract(nodes, base_url):
    out = []
    seen = set()
    for node in nodes:
        title = node.get_text(" ", strip=True)
        link = node if (node.name == "a" and node.get("href")) else node.find("a", href=True)
        href = _abs(base_url, link["href"]) if link else base_url
        key = (title, href)
        if not title or key in seen:
            continue
        seen.add(key)
        out.append({"title": title[:500], "url": href})
    return out


def _score_group(nodes, base_url):
    """Score how much a group of sibling elements looks like a job list.

    Rejects (returns None) anything that reads as navigation / related links
    rather than postings: cards without their own link, links to other hosts,
    or links that neither look job-ish nor share a common path.
    """
    n = len(nodes)
    if n < MIN_ITEMS:
        return None
    base_host = (urlparse(base_url).hostname or "").lower()

    hrefs, texts = [], []
    for node in nodes:
        link = node if (node.name == "a" and node.get("href")) else node.find("a", href=True)
        if link:
            hrefs.append(_abs(base_url, link["href"]))
        texts.append(node.get_text(" ", strip=True))

    if len(hrefs) < n * 0.8:                       # most cards must carry a link
        return None
    distinct = len(set(hrefs))
    if distinct < max(MIN_ITEMS, int(n * 0.8)):    # and mostly distinct ones
        return None

    same_host = sum(
        (urlparse(h).hostname or base_host).lower() == base_host for h in hrefs
    ) / len(hrefs)
    if same_host < 0.6:                            # nav to youtube/twitter/etc.
        return None

    low_hrefs = [h.lower() for h in hrefs]
    href_hint = sum(any(k in h for k in JOB_HREF_HINTS) for h in low_hrefs) / len(hrefs)
    seg_prefix = ["/".join(urlparse(h).path.split("/")[:3]) for h in hrefs]
    prefix_frac = Counter(seg_prefix).most_common(1)[0][1] / len(seg_prefix)
    if href_hint < 0.3 and prefix_frac < 0.6:      # not a coherent job list
        return None

    avg_len = sum(len(t) for t in texts) / n
    if avg_len < 8 or avg_len > 1200:
        return None
    text_hint = sum(any(k in t.lower() for k in JOB_TEXT_HINTS) for t in texts) / n
    if text_hint < MIN_TEXT_HINT:                  # footers, tag clouds, filters
        return None

    score = min(n, 25)                             # "enough items", capped low
    score += 40 * href_hint
    score += 25 * prefix_frac
    score += 20 * text_hint
    score += 10 if distinct == n else 0
    return score


def detect_listings(html, url):
    """Return {selector, count, listings, alternatives, note} or None."""
    host = (urlparse(url).hostname or "").lower()
    soup = BeautifulSoup(html, "html.parser")
    for tag in soup(["script", "style", "noscript", "svg", "template"]):
        tag.decompose()

    # 1. known ATS
    for marker, selector in KNOWN_ATS.items():
        if marker in host:
            if selector is None:
                return {
                    "selector": None, "count": 0, "listings": [], "alternatives": [],
                    "note": f"{marker} renders jobs with JavaScript; static fetch can't see them.",
                }
            nodes = soup.select(selector)
            listings = _extract(nodes, url)
            if listings:
                return {
                    "selector": selector, "count": len(listings), "listings": listings,
                    "alternatives": [], "note": f"matched known pattern for {marker}",
                }
            break

    # 2. generic: group anchor ancestors by (parent, signature)
    groups = {}
    for a in soup.find_all("a", href=True):
        href = a["href"].strip()
        if not href or href.startswith(("#", "javascript:", "mailto:", "tel:")):
            continue
        node = a
        for _ in range(MAX_CLIMB):
            parent = node.parent
            if parent is None or parent.name in ("body", "html", "[document]"):
                break
            key = (id(parent), _sig(node))
            group = groups.setdefault(key, {"parent": parent, "sig": _sig(node), "nodes": []})
            if node not in group["nodes"]:
                group["nodes"].append(node)
            node = parent

    scored = []
    for group in groups.values():
        score = _score_group(group["nodes"], url)
        if score is not None:
            scored.append((score, group))

    if not scored:
        note = "no repeating job list found"
        if _looks_js_rendered(html):
            note += "; page appears to be JavaScript-rendered"
        return {"selector": None, "count": 0, "listings": [], "alternatives": [], "note": note}

    scored.sort(key=lambda s: s[0], reverse=True)
    best_score, best = scored[0]

    # On a JS-rendered page any static match is more likely to be chrome
    # (footer, nav) than the real list, so demand a much stronger signal.
    js = _looks_js_rendered(html)
    threshold = MIN_CONFIDENCE * 1.6 if js else MIN_CONFIDENCE
    if best_score < threshold:
        note = f"no confident match (best score {best_score:.0f})"
        if js:
            note += "; page appears to be JavaScript-rendered"
        return {"selector": None, "count": 0, "listings": [], "alternatives": [], "note": note}

    selector = _selector_for(best["parent"], best["sig"])
    listings = _extract(best["nodes"], url)
    alternatives = []
    for _, group in scored[1:4]:
        alt = _selector_for(group["parent"], group["sig"])
        if alt != selector and alt not in alternatives:
            alternatives.append(alt)

    return {
        "selector": selector,
        "count": len(listings),
        "listings": listings,
        "alternatives": alternatives,
        "note": f"auto-detected (score {best_score:.0f}, {len(listings)} listings)",
    }
