package fetcher

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

const (
	PlatformTalentBrew      = "talentbrew"
	PlatformGreenhouse      = "greenhouse"
	PlatformWorkday         = "workday"
	PlatformLever           = "lever"
	PlatformAshby           = "ashby"
	PlatformSmartRecruiters = "smartrecruiters"
	PlatformWorkable        = "workable"
	PlatformRecruitee       = "recruitee"
	PlatformBeeSite         = "beesite"
)

var (
	// ErrMissingBoard means a platform that needs a board name has none to look up.
	ErrMissingBoard = errors.New("board name is required")
	// ErrBoardNotFound means the platform has no job board with that name.
	ErrBoardNotFound = errors.New("job board not found")
	// ErrWorkdaySiteNotFound means the Workday tenant has no career site with that name.
	ErrWorkdaySiteNotFound = errors.New("workday career site not found")
)

// IsKnownPlatform reports whether p is a platform value companies may store.
func IsKnownPlatform(p string) bool {
	switch p {
	case PlatformTalentBrew, PlatformGreenhouse, PlatformWorkday, PlatformLever,
		PlatformAshby, PlatformSmartRecruiters, PlatformWorkable, PlatformRecruitee, PlatformBeeSite:
		return true
	}
	return false
}

// NeedsBoard reports whether reading the platform requires a board name (the
// company's slug on it). TalentBrew and Workday work from the career URL alone.
func NeedsBoard(platform string) bool {
	return IsKnownPlatform(platform) && platform != PlatformTalentBrew && platform != PlatformWorkday
}

// DetectPlatform guesses the platform (and board name) from the URL alone.
// It returns "" when the URL doesn't reveal it, e.g. a company hosting its
// own careers page in front of Greenhouse.
func DetectPlatform(careerURL string) (platform, board string) {
	u, err := url.Parse(careerURL)
	if err != nil {
		return "", ""
	}

	host := strings.ToLower(u.Hostname())
	firstSegment := func() string {
		return strings.Split(strings.Trim(u.Path, "/"), "/")[0]
	}

	switch {
	case host == "boards.greenhouse.io" || host == "job-boards.greenhouse.io":
		if firstSegment() == "embed" {
			return PlatformGreenhouse, u.Query().Get("for")
		}
		return PlatformGreenhouse, firstSegment()
	case strings.HasSuffix(host, ".myworkdayjobs.com"):
		return PlatformWorkday, ""
	case host == "jobs.lever.co":
		return PlatformLever, firstSegment()
	case host == "jobs.ashbyhq.com":
		return PlatformAshby, firstSegment()
	case host == "jobs.smartrecruiters.com" || host == "careers.smartrecruiters.com":
		return PlatformSmartRecruiters, firstSegment()
	case host == "apply.workable.com":
		return PlatformWorkable, firstSegment()
	case strings.HasSuffix(host, ".recruitee.com"):
		sub := strings.TrimSuffix(host, ".recruitee.com")
		if sub == "www" || sub == "api" || sub == "app" || strings.Contains(sub, ".") {
			return "", ""
		}
		return PlatformRecruitee, sub
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
	case PlatformLever:
		return FetchLeverJobs(ctx, board)
	case PlatformAshby:
		return FetchAshbyJobs(ctx, board)
	case PlatformSmartRecruiters:
		return FetchSmartRecruitersJobs(ctx, board, maxPages)
	case PlatformWorkable:
		return FetchWorkableJobs(ctx, board)
	case PlatformRecruitee:
		return FetchRecruiteeJobs(ctx, board)
	case PlatformBeeSite:
		return FetchBeeSiteJobs(ctx, board, maxPages)
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
