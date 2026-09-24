package fetcher

import "strings"

// FilterJobs keeps the jobs that match ANY of the roles AND ANY of the
// locations. An empty list means "no constraint" for that dimension.
//
// Roles are matched as case-insensitive substrings of the job title.
// Locations are matched against the job's location (see matchesLocation).
func FilterJobs(jobs []Job, roles, locations []string) []Job {
	matched := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		if matchesAnyRole(j, roles) && matchesAnyLocation(j, locations) {
			matched = append(matched, j)
		}
	}
	return matched
}

func matchesAnyRole(j Job, roles []string) bool {
	if len(roles) == 0 {
		return true
	}
	title := strings.ToLower(j.Title)
	for _, role := range roles {
		if strings.Contains(title, strings.ToLower(role)) {
			return true
		}
	}
	return false
}

func matchesAnyLocation(j Job, locations []string) bool {
	if len(locations) == 0 {
		return true
	}
	for _, loc := range locations {
		if matchesLocation(j, loc) {
			return true
		}
	}
	return false
}

// matchesLocation splits a filter like "Bangalore, India" into its
// comma-separated parts and requires every part to appear in the job's
// location. Whole-string matching would miss "Bangalore, Karnātaka, India",
// which is how the site actually writes it. "Remote" also counts if it
// appears in the title, since remote roles often aren't tagged in the
// location field.
func matchesLocation(j Job, filter string) bool {
	location := strings.ToLower(j.Location)
	title := strings.ToLower(j.Title)

	hasPart := false
	for _, part := range strings.Split(strings.ToLower(filter), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		hasPart = true
		if strings.Contains(location, part) {
			continue
		}
		if part == "remote" && strings.Contains(title, part) {
			continue
		}
		return false
	}
	return hasPart
}
