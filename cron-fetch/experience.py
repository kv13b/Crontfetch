"""Parse a 'years of experience' requirement out of free job-listing text.

parse_experience(text) -> (min_years, max_years) | None
  - min_years is always a number (0 when only an upper bound is stated)
  - max_years is None when the requirement is open-ended ("5+ years")
  - returns None when no experience requirement could be found

experience_matches(user_years, parsed) -> bool
  - True when the user's years fall inside the parsed band
  - False when parsed is None (unknown experience is skipped by design)
"""
import re

_NUM = r"(\d+(?:\.\d+)?)"
_YEARS = r"(?:\+\s*)?(?:years?|yrs?|yr)\b"

# Order matters: the first pattern that matches wins.
_RANGE = re.compile(rf"{_NUM}\s*(?:-|–|—|to)\s*{_NUM}\s*{_YEARS}", re.I)
_PLUS = re.compile(rf"{_NUM}\s*\+\s*(?:years?|yrs?|yr)\b", re.I)
_MIN = re.compile(rf"(?:min(?:imum)?|at\s*least|atleast|over|more\s*than)\s*(?:of\s*)?{_NUM}\s*{_YEARS}", re.I)
_MAX = re.compile(rf"(?:up\s*to|max(?:imum)?|less\s*than|under|below)\s*{_NUM}\s*{_YEARS}", re.I)
_EXACT = re.compile(rf"{_NUM}\s*{_YEARS}", re.I)
_FRESHER = re.compile(r"\b(fresher|entry[\s-]*level|no\s+experience|0\s*(?:years?|yrs?))\b", re.I)


def parse_experience(text):
    if not text:
        return None
    t = text.replace("\xa0", " ")

    m = _RANGE.search(t)
    if m:
        lo, hi = sorted((float(m.group(1)), float(m.group(2))))
        return (lo, hi)

    m = _PLUS.search(t)
    if m:
        return (float(m.group(1)), None)

    m = _MIN.search(t)
    if m:
        return (float(m.group(1)), None)

    m = _MAX.search(t)
    if m:
        return (0.0, float(m.group(1)))

    if _FRESHER.search(t):
        return (0.0, 1.0)

    m = _EXACT.search(t)
    if m:
        n = float(m.group(1))
        return (n, n)

    return None


def experience_matches(user_years, parsed):
    """User has `user_years` of experience; does the parsed requirement fit?"""
    if parsed is None or user_years is None:
        return False
    lo, hi = parsed
    lo = lo or 0.0
    if user_years < lo:
        return False
    if hi is not None and user_years > hi:
        return False
    return True


if __name__ == "__main__":
    samples = [
        "Senior Engineer - 5+ years of experience required",
        "Backend Developer (2-4 years)",
        "Data Analyst, minimum 3 years experience",
        "Frontend Engineer up to 2 years",
        "Software Engineer - Fresher",
        "Product Manager with 7 years experience",
        "Marketing Lead",
    ]
    for s in samples:
        print(f"{parse_experience(s)!s:>16}  <-  {s}")
