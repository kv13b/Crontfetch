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
)

var (
	// ErrMissingBoard means a Greenhouse company has no board name to look up.
	ErrMissingBoard = errors.New("greenhouse board name is required")
	// ErrBoardNotFound means Greenhouse has no job board with that name.
	ErrBoardNotFound = errors.New("greenhouse board not found")
)

// IsKnownPlatform reports whether p is a platform value companies may store.
func IsKnownPlatform(p string) bool {
	return p == PlatformTalentBrew || p == PlatformGreenhouse
}

// DetectPlatform guesses the platform (and Greenhouse board name) from the
// URL alone. It returns "" when the URL doesn't reveal it, e.g. a company
// hosting its own careers page in front of Greenhouse.
func DetectPlatform(careerURL string) (platform, board string) {
	u, err := url.Parse(careerURL)
	if err != nil {
		return "", ""
	}

	switch strings.ToLower(u.Hostname()) {
	case "boards.greenhouse.io", "job-boards.greenhouse.io":
		segments := strings.Split(strings.Trim(u.Path, "/"), "/")
		if segments[0] == "embed" {
			return PlatformGreenhouse, u.Query().Get("for")
		}
		return PlatformGreenhouse, segments[0]
	}
	return "", ""
}

// FetchJobs picks a fetcher for the company and returns its listings.
// platform and board are the company's saved overrides; anything empty is
// filled in from the URL. A URL that matches no known platform is tried as
// TalentBrew, which returns ErrUnsupportedPlatform if it isn't one either.
func FetchJobs(ctx context.Context, careerURL, platform, board string, maxPages int) ([]Job, error) {
	detectedPlatform, detectedBoard := DetectPlatform(careerURL)
	if platform == "" {
		platform = detectedPlatform
	}
	if board == "" {
		board = detectedBoard
	}

	switch platform {
	case PlatformGreenhouse:
		return FetchGreenhouseJobs(ctx, board)
	default:
		return FetchTalentBrewJobs(ctx, careerURL, maxPages)
	}
}
