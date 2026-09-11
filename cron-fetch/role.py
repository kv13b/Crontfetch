"""Match a job listing's text against a user-selected list of role keywords.

Same plain case-insensitive substring test as location.py: a listing matches
when any of the selected role strings appears anywhere in its text (title,
department tag, etc - whatever the item selector captured), e.g.:

    "Senior IT Software Engineer  Santa Clara, California  IT"        <- "Engineer"
    "Backend Developer (2-4 years)  Bengaluru, India  Engineering"    <- "Engineering"
    "Sr Account Rep  San Diego, California  Sales"                    <- no match

parse_roles(raw) -> list[str] | None
  - accepts a list (["Engineering", "Software Developer"]) or a
    comma-separated string; returns None when no filter is wanted.

find_role(roles, text) -> str | None
  - returns whichever entry from `roles` was found in `text`, or None.
"""


def parse_roles(raw):
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


def find_role(roles, text):
    if not roles or not text:
        return None
    low = text.lower()
    for role in roles:
        if role.lower() in low:
            return role
    return None


def role_matches(roles, text):
    """True when no filter is set, or when one of `roles` is found in `text`."""
    if not roles:
        return True
    return find_role(roles, text) is not None


if __name__ == "__main__":
    samples = [
        "Senior IT Software Engineer Santa Clara, California IT",
        "Backend Developer (2-4 years) Bengaluru, India Engineering",
        "Sr Account Rep San Diego, California Sales",
        "Software Developer II Remote",
    ]
    wanted = parse_roles(["Engineering", "Software Developer"])
    for s in samples:
        print(f"{find_role(wanted, s)!s:>18}  <-  {s}")
