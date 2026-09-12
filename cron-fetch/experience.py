"""Parse a 'years of experience' requirement out of free job-listing text.

parse_experience(text) -> (min_years, max_years) | None
  - min_years is always a number (0 when only an upper bound is stated)
  - max_years is None when the requirement is open-ended ("5+ years")
  - returns None when no experience requirement could be found

experience_matches(min_years, max_years, parsed) -> bool
  - min_years / max_years describe the band of experience you're open to; a
    listing matches when its own (job_min, job_max) range overlaps that band.
  - either bound may be None: only min_years given -> band is the single
    point [min_years, min_years] (matches jobs pitched at exactly that level);
    only max_years given -> band is [0, max_years] ("nothing too senior");
    neither given -> no filter, always matches.
  - False when parsed is None and a filter is set (unknown experience is
    skipped by design); True when parsed is None and no filter is set.
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


def experience_matches(min_years, max_years, parsed):
    """Does the parsed job requirement overlap the [min_years, max_years] band?

    See the module docstring for how a missing bound is defaulted. This is a
    strict generalisation of the old single-value check: passing only
    `min_years` reproduces the original point-match behaviour exactly.
    """
    if min_years is None and max_years is None:
        return True
    if parsed is None:
        return False

    band_lo = 0.0 if min_years is None else min_years
    band_hi = min_years if max_years is None else max_years

    job_lo, job_hi = parsed
    job_lo = job_lo or 0.0
    if job_lo > band_hi:
        return False
    if job_hi is not None and job_hi < band_lo:
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

    print()
    bands = [(3, None), (None, 3), (2, 5)]
    jobs = [(2.0, 4.0), (5.0, None), (0.0, 1.0), (6.0, 8.0)]
    for band in bands:
        row = [experience_matches(*band, job) for job in jobs]
        print(f"band {band!s:>10}  vs jobs {jobs} -> {row}")
