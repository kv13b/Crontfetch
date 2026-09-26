package fetcher

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

const (
	PlatformTalentBrew = "talentbrew"
	PlatformGreenhouse = "greenhouse"
	PlatformWorkday    = "workday"
)

var (
	// ErrMissingBoard means a Greenhouse company has no board name to look up.
	ErrMissingBoard = errors.New("greenhouse board name is required")
	// ErrBoardNotFound means Greenhouse has no job board with that name.
	ErrBoardNotFound = errors.New("greenhouse board not found")
	// ErrWorkdaySiteNotFound means the Workday tenant has no career site with that name.
	ErrWorkdaySiteNotFound = errors.New("workday career site not found")
)

// IsKnownPlatform reports whether p is a platform value companies may store.
func IsKnownPlatform(p string) bool {
	return p == PlatformTalentBrew || p == PlatformGreenhouse || p == PlatformWorkday
}

// DetectPlatform guesses the platform (and Greenhouse board name) from the
// URL alone. It returns "" when the URL doesn't reveal it, e.g. a company
// hosting its own careers page in front of Greenhouse.
func DetectPlatform(careerURL string) (platform, board string) {
	u, err := url.Parse(careerURL)
	if err != nil {
		return "", ""
	}

	host := strings.ToLower(u.Hostname())
	switch {
	case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io":
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if segments[0] == "embed" {
			return PlatformGreenhouse, u.Query().Get("for")
		}
		return PlatformGreenhouse, segments[0]
	case strings.HasSuffix(host, ".myworkdayjobs.com"):
		return PlatformWorkday, ""
	}
	return "", ""
}

// FetchJobs picks a fetcher for the company and returns its listings.
// platform and board are what was saved for the company (see Detect); when
// no platform is saved, it is detected now. If nothing is recognised, the
// page is tried as TalentBrew, which returns ErrUnsupportedPlatform if it
// isn't one either.
func FetchJobs(ctx context.Context, careerURL, platform, board string, maxPages int) ([]Job, error) {
	if platform == "" {
		detected := Detect(ctx, careerURL)
		platform = detected.Platform
		if board == "" {
			board = detected.Board
		}
	}

	switch platform {
	case PlatformGreenhouse:
		return FetchGreenhouseJobs(ctx, board)
	case PlatformWorkday:
		workdayURL := careerURL
		if board != "" {
			workdayURL = board
		}
		return FetchWorkdayJobs(ctx, workdayURL, maxPages)
	case PlatformTalentBrew, "":
		return FetchTalentBrewJobs(ctx, careerURL, maxPages)
	default:
		return nil, &UnsupportedPlatformError{Platform: platform}
	}
}
