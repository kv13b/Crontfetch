package fetcher

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
)

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// stripHTML removes tags and unescapes entities, leaving plain text that's
// safe to run the patterns below against.
func stripHTML(s string) string {
	return html.UnescapeString(htmlTagRe.ReplaceAllString(s, " "))
}

// Patterns are tried in this order — most specific (an explicit range) to
// least (a bare number near the word "experience") — and the first one that
// matches anywhere in the text wins, since a posting that states its
// requirement clearly usually does so once, near the top.
var (
	expRangeRe   = regexp.MustCompile(`(?i)(\d{1,2})\s*(?:-|–|—|to)\s*(\d{1,2})\+?\s*years?`)
	expUpToRe    = regexp.MustCompile(`(?i)up to\s*(\d{1,2})\s*years?`)
	expPlusRe    = regexp.MustCompile(`(?i)(\d{1,2})\s*\+\s*years?`)
	expAtLeastRe = regexp.MustCompile(`(?i)(?:at least|minimum(?: of)?)\s*(\d{1,2})\s*years?`)
	expBareRe    = regexp.MustCompile(`(?i)(\d{1,2})\s*years?'?\s*(?:of\s+)?(?:experience|exp\b)`)
)

// parsedExperience is the numeric year range a job posting states, if any.
// A nil min or max means that side is unbounded (e.g. "5+ years" has no
// stated upper bound; "up to 3 years" has no stated lower bound).
type parsedExperience struct {
	found   bool
	min     *int
	max     *int
	display string
}

func intPtr(n int) *int { return &n }

// parseExperience looks for a stated years-of-experience requirement in
// free-form text. It's a best-effort heuristic over prose written by many
// different people, not a guarantee: unusual phrasing ("several years",
// "fresh graduates welcome", a range given in words) won't be recognized.
// found=false means "couldn't tell", not "no experience required".
func parseExperience(text string) parsedExperience {
	if m := expRangeRe.FindStringSubmatch(text); m != nil {
		lo, _ := strconv.Atoi(m[1])
		hi, _ := strconv.Atoi(m[2])
		if lo > hi {
			lo, hi = hi, lo
		}
		return parsedExperience{true, intPtr(lo), intPtr(hi), fmt.Sprintf("%d-%d years", lo, hi)}
	}
	if m := expUpToRe.FindStringSubmatch(text); m != nil {
		hi, _ := strconv.Atoi(m[1])
		return parsedExperience{true, intPtr(0), intPtr(hi), fmt.Sprintf("up to %d years", hi)}
	}
	if m := expPlusRe.FindStringSubmatch(text); m != nil {
		lo, _ := strconv.Atoi(m[1])
		return parsedExperience{true, intPtr(lo), nil, fmt.Sprintf("%d+ years", lo)}
	}
	if m := expAtLeastRe.FindStringSubmatch(text); m != nil {
		lo, _ := strconv.Atoi(m[1])
		return parsedExperience{true, intPtr(lo), nil, fmt.Sprintf("%d+ years", lo)}
	}
	if m := expBareRe.FindStringSubmatch(text); m != nil {
		lo, _ := strconv.Atoi(m[1])
		return parsedExperience{true, intPtr(lo), nil, fmt.Sprintf("%d+ years", lo)}
	}
	return parsedExperience{}
}

// SupportsExperienceFilter reports whether a platform's job listings include
// enough text to look for a stated experience range. Workday,
// SmartRecruiters, TalentBrew and BeeSite don't include a full description
// in their listing responses (only in Workday's and TalentBrew's case would
// getting one mean an extra request per job), so experience filtering can't
// apply to them yet.
func SupportsExperienceFilter(platform string) bool {
	switch platform {
	case PlatformGreenhouse, PlatformLever, PlatformAshby, PlatformWorkable, PlatformRecruitee:
		return true
	}
	return false
}

// withExperience parses description (HTML or plain text) for a stated
// experience range and returns a copy of j with it attached, if found.
func (j Job) withExperience(description string) Job {
	if parsed := parseExperience(stripHTML(description)); parsed.found {
		j.Experience = parsed.display
		j.expMin = parsed.min
		j.expMax = parsed.max
	}
	return j
}
