package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// maxDetectBody bounds how much of a career page is read while looking for
// platform markers.
const maxDetectBody = 2 << 20

// Detection is the result of working out which platform a career page uses.
type Detection struct {
	// Platform is a supported platform constant, the name of a recognised
	// but unsupported vendor (e.g. "lever"), or "" when nothing was found.
	Platform string
	// Board is the Greenhouse board name, or for Workday the full URL of
	// its career site (a company's own page usually isn't that URL).
	Board string
}

// Usable reports whether the result can be fetched from as-is.
func (d Detection) Usable() bool {
	if !IsKnownPlatform(d.Platform) {
		return false
	}
	return d.Platform != PlatformGreenhouse || d.Board != ""
}

// UnsupportedPlatformError says which recognised vendor a career site uses
// when we can't read it yet. It matches ErrUnsupportedPlatform.
type UnsupportedPlatformError struct{ Platform string }

func (e *UnsupportedPlatformError) Error() string {
	return fmt.Sprintf("career site uses %s, which is not supported yet", e.Platform)
}

func (e *UnsupportedPlatformError) Unwrap() error { return ErrUnsupportedPlatform }

var (
	greenhouseBoardRe = regexp.MustCompile(`(?i)(?:boards|job-boards)\.greenhouse\.io/(?:embed/job_board(?:/js)?\?for=)?([a-z0-9_-]+)`)
	greenhouseAPIRe   = regexp.MustCompile(`(?i)boards-api\.greenhouse\.io/v1/boards/([a-z0-9_-]+)`)
	workdaySiteRe     = regexp.MustCompile(`(?i)https?://([a-z0-9-]+\.wd\d+\.myworkdayjobs\.com)/(?:[a-z]{2}-[A-Za-z]{2}/)?([A-Za-z0-9_-]+)`)

	// Vendors we recognise but can't read yet, so the user gets a precise
	// "uses Lever" message instead of a generic failure.
	unsupportedVendors = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"lever", regexp.MustCompile(`(?i)jobs\.(eu\.)?lever\.co/|api\.lever\.co/`)},
		{"ashby", regexp.MustCompile(`(?i)jobs\.ashbyhq\.com/`)},
		{"smartrecruiters", regexp.MustCompile(`(?i)smartrecruiters\.com/`)},
		{"workable", regexp.MustCompile(`(?i)apply\.workable\.com/`)},
		{"recruitee", regexp.MustCompile(`(?i)\.recruitee\.com`)},
		{"teamtailor", regexp.MustCompile(`(?i)\.teamtailor\.com`)},
		{"personio", regexp.MustCompile(`(?i)\.jobs\.personio\.`)},
		{"jobvite", regexp.MustCompile(`(?i)jobs\.jobvite\.com`)},
		{"icims", regexp.MustCompile(`(?i)\.icims\.com`)},
		{"taleo", regexp.MustCompile(`(?i)\.taleo\.net`)},
		{"successfactors", regexp.MustCompile(`(?i)successfactors\.(com|eu)|\.sapsf\.(com|eu)`)},
		{"oracle", regexp.MustCompile(`(?i)oraclecloud\.com/hcmUI/CandidateExperience`)},
	}
)

// Detect works out which platform a career URL belongs to, in order of
// cost: the URL itself, markers in the page's HTML, then a check of whether
// a Greenhouse board is named after the company's domain.
//
// It can miss: pages that load everything with JavaScript and leave no
// marker, and Greenhouse boards not named after the domain.
func Detect(ctx context.Context, careerURL string) Detection {
	if platform, board := DetectPlatform(careerURL); platform != "" {
		return Detection{Platform: platform, Board: board}
	}

	var page Detection
	if html, ok := fetchPage(ctx, careerURL); ok {
		page = scanPage(html)
	}
	if page.Usable() {
		return page
	}

	// A recognised unsupported vendor is real evidence; a guess shouldn't
	// override it. With no evidence, or Greenhouse without a board name,
	// try naming the board after the company.
	if page.Platform == "" || page.Platform == PlatformGreenhouse {
		if board := guessGreenhouseBoard(ctx, careerURL); board != "" {
			return Detection{Platform: PlatformGreenhouse, Board: board}
		}
	}
	return page
}

// scanPage looks for platform markers in a page's HTML. When several
// platforms appear, "the page is built on X" (TalentBrew's asset host)
// outranks "the page links to X": Palo Alto's TalentBrew site also links
// to a Workday page, but its jobs are on TalentBrew.
func scanPage(html string) Detection {
	lower := strings.ToLower(html)

	if strings.Contains(lower, "talentbrew.com") {
		return Detection{Platform: PlatformTalentBrew}
	}

	for _, re := range []*regexp.Regexp{greenhouseAPIRe, greenhouseBoardRe} {
		for _, m := range re.FindAllStringSubmatch(html, -1) {
			if board := strings.ToLower(m[1]); board != "embed" && board != "v1" {
				return Detection{Platform: PlatformGreenhouse, Board: board}
			}
		}
	}

	for _, m := range workdaySiteRe.FindAllStringSubmatch(html, -1) {
		switch strings.ToLower(m[2]) {
		case "wday", "assets", "static", "login":
			continue
		}
		return Detection{Platform: PlatformWorkday, Board: "https://" + strings.ToLower(m[1]) + "/" + m[2]}
	}

	// Greenhouse's job-link tracking parameter, without a visible board name.
	if strings.Contains(lower, "gh_jid=") {
		return Detection{Platform: PlatformGreenhouse}
	}

	for _, v := range unsupportedVendors {
		if v.re.MatchString(html) {
			return Detection{Platform: v.name}
		}
	}
	return Detection{}
}

func fetchPage(ctx context.Context, pageURL string) (string, bool) {
	resp, err := get(ctx, pageURL, false)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDetectBody))
	if err != nil {
		return "", false
	}
	return string(body), true
}

// guessGreenhouseBoard tries the company's domain name as a Greenhouse
// board and accepts it only if the board's registered name matches, so an
// unrelated company that happens to own that board name isn't mistaken
// for this one.
func guessGreenhouseBoard(ctx context.Context, careerURL string) string {
	u, err := url.Parse(careerURL)
	if err != nil {
		return ""
	}
	label := companyLabel(u.Hostname())
	if label == "" {
		return ""
	}

	resp, err := get(ctx, greenhouseAPIBase+"/"+url.PathEscape(label), false)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var board struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&board); err != nil {
		return ""
	}
	if !namesMatch(board.Name, label) {
		return ""
	}
	return label
}

// companyLabel returns the registrable name from a hostname:
// "www.valtech.com" -> "valtech", "careers.acme.co.uk" -> "acme".
func companyLabel(host string) string {
	host = strings.ToLower(host)
	if net.ParseIP(host) != nil {
		return ""
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return ""
	}

	i := len(labels) - 2
	if len(labels) >= 3 && len(labels[len(labels)-1]) == 2 {
		switch labels[len(labels)-2] {
		case "co", "com", "org", "net", "gov", "ac", "edu":
			i = len(labels) - 3
		}
	}
	return labels[i]
}

// namesMatch compares two names ignoring case and punctuation, accepting
// one containing the other ("Valtech" / "valtech", "Palo Alto Networks" /
// "paloaltonetworks").
func namesMatch(a, b string) bool {
	a, b = alnum(a), alnum(b)
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func alnum(s string) string {
	var out strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
	}
	return out.String()
}
