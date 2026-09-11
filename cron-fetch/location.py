"""Match a job listing's text against a user-selected list of locations.

Matching is a plain case-insensitive substring test: a listing matches when
any of the selected location strings appears anywhere in its text. This works
without any special parsing because job cards typically embed the location
right in their text, e.g.:

    "Senior Engineer  Bengaluru, Karnataka, India  Engineering"
    "MDR Shift Analyst  WEST COAST, REMOTE  Santa Clara, California  InfoSec"

parse_locations(raw) -> list[str] | None
  - accepts a list (["India", "Remote"]) or a comma-separated string
    ("India, Remote") from the API; returns None when no filter is wanted.

find_location(locations, text) -> str | None
  - returns whichever entry from `locations` was found in `text`, or None.
    None also means "no filter set" upstream (locations is falsy) should be
    treated as "matches everything" - callers check `if locations` first.
"""


def parse_locations(raw):
    if raw in (None, "", []):
        return None
    if isinstance(raw, str):
        raw = raw.split(",")

    cleaned, seen = [], set()
    for item in raw:
        text = str(item).strip()
        if text and text.lower() not in seen:
            seen.add(text.lower())
            cleaned.append(text)
    return cleaned or None


def find_location(locations, text):
    if not locations or not text:
        return None
    low = text.lower()
    for loc in locations:
        if loc.lower() in low:
            return loc
    return None


def location_matches(locations, text):
    """True when no filter is set, or when one of `locations` is found in `text`."""
    if not locations:
        return True
    return find_location(locations, text) is not None


if __name__ == "__main__":
    samples = [
        "Senior Engineer Bengaluru, Karnataka, India Engineering",
        "MDR Shift Analyst WEST COAST, REMOTE Santa Clara, California InfoSec",
        "Sr Account Rep San Diego, California, United States of America Sales",
        "Backend Developer Remote (India preferred)",
    ]
    wanted = parse_locations(["India", "Remote"])
    for s in samples:
        print(f"{find_location(wanted, s)!s:>8}  <-  {s}")
