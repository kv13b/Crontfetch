package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/sync/errgroup"
)

// workdayPageSize is Workday's maximum: a larger limit is rejected with 400.
const workdayPageSize = 20

// workdayLocale matches URL segments like "en-US" so they aren't mistaken
// for the career site's name.
var workdayLocale = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2})?$`)

type workdaySite struct {
	Host   string // e.g. nvidia.wd5.myworkdayjobs.com
	Tenant string // e.g. nvidia
	Site   string // e.g. NVIDIAExternalCareerSite
}

// workdayAPIURL is a variable so tests can point it at a local server.
var workdayAPIURL = func(s workdaySite) string {
	return "https://" + s.Host + "/wday/cxs/" + s.Tenant + "/" + s.Site + "/jobs"
}

// parseWorkdayURL pulls the tenant and site name out of a Workday career
// URL such as https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite.
func parseWorkdayURL(careerURL string) (workdaySite, bool) {
	u, err := url.Parse(careerURL)
	if err != nil {
		return workdaySite{}, false
	}
	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".myworkdayjobs.com") {
		return workdaySite{}, false
	}

	for _, segment := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if segment == "" || workdayLocale.MatchString(segment) {
			continue
		}
		return workdaySite{Host: host, Tenant: strings.Split(host, ".")[0], Site: segment}, true
	}
	return workdaySite{}, false
}

type workdayPage struct {
	Total       int `json:"total"`
	JobPostings []struct {
		Title         string   `json:"title"`
		ExternalPath  string   `json:"externalPath"`
		LocationsText string   `json:"locationsText"`
		BulletFields  []string `json:"bulletFields"`
	} `json:"jobPostings"`
}

// FetchWorkdayJobs reads a company's Workday career site through the JSON
// endpoint its own page uses. Everything needed (tenant, site) comes from
// the URL. maxPages caps how many pages (20 jobs each) are read.
//
// Jobs with several locations only say "2 Locations" in the listing, so
// location filters can't match them without reading each job's detail page.
func FetchWorkdayJobs(ctx context.Context, careerURL string, maxPages int) ([]Job, error) {
	site, ok := parseWorkdayURL(careerURL)
	if !ok {
		return nil, ErrUnsupportedPlatform
	}
	endpoint := workdayAPIURL(site)

	first, err := fetchWorkdayPage(ctx, endpoint, 0)
	if err != nil {
		return nil, err
	}

	// Workday only reports the total on the first page; later pages say 0.
	totalPages := (first.Total + workdayPageSize - 1) / workdayPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if maxPages > 0 && totalPages > maxPages {
		totalPages = maxPages
	}

	// Indexed by page so results stay in order despite concurrent fetching.
	pages := make([]workdayPage, totalPages)
	pages[0] = first

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentPages)
	for page := 1; page < totalPages; page++ {
		g.Go(func() error {
			p, err := fetchWorkdayPage(gctx, endpoint, page*workdayPageSize)
			if err != nil {
				return fmt.Errorf("page %d: %w", page+1, err)
			}
			pages[page] = p
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Jobs posted mid-scan can shift the offsets and repeat a listing.
	seen := make(map[string]bool)
	var jobs []Job
	for _, p := range pages {
		for _, posting := range p.JobPostings {
			id := posting.ExternalPath
			if len(posting.BulletFields) > 0 {
				id = posting.BulletFields[0]
			}
			if seen[id] {
				continue
			}
			seen[id] = true

			jobs = append(jobs, Job{
				ID:       id,
				Title:    strings.TrimSpace(posting.Title),
				Location: strings.TrimSpace(posting.LocationsText),
				URL:      "https://" + site.Host + "/" + site.Site + posting.ExternalPath,
			})
		}
	}
	return jobs, nil
}

func fetchWorkdayPage(ctx context.Context, endpoint string, offset int) (workdayPage, error) {
	body, err := json.Marshal(map[string]any{
		"appliedFacets": map[string]any{},
		"limit":         workdayPageSize,
		"offset":        offset,
		"searchText":    "",
	})
	if err != nil {
		return workdayPage{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return workdayPage{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return workdayPage{}, fmt.Errorf("fetching workday jobs: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return workdayPage{}, ErrWorkdaySiteNotFound
	case resp.StatusCode != http.StatusOK:
		return workdayPage{}, fmt.Errorf("workday returned status %d", resp.StatusCode)
	}

	var page workdayPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return workdayPage{}, fmt.Errorf("decoding workday response: %w", err)
	}
	return page, nil
}
