// Package fetcher scrapes job listings from company career pages.
package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/sync/errgroup"
)

// Job is a single job listing pulled off a career page.
type Job struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Location string `json:"location"`
	URL      string `json:"url"`
}

// ErrUnsupportedPlatform means the career page isn't built on a platform
// this package knows how to read (as opposed to a page that simply has no jobs).
var ErrUnsupportedPlatform = errors.New("career page is not on a supported platform")

// maxConcurrentPages caps simultaneous requests to a single career site so
// a full scan doesn't hammer it.
const maxConcurrentPages = 8

// httpClient has a timeout, unlike http.DefaultClient, which would let a
// stalled career site hang our request forever.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// FetchTalentBrewJobs scrapes job listings from a career page built on the
// TalentBrew/Radancy platform (e.g. Palo Alto Networks). careerURL is the
// base career page, such as "https://jobs.paloaltonetworks.com/en"; the
// search pages and job links are derived from it, so any company on this
// platform works. maxPages caps how many result pages (15 jobs each) are read.
//
// Returns ErrUnsupportedPlatform if the page isn't TalentBrew-based.
func FetchTalentBrewJobs(ctx context.Context, careerURL string, maxPages int) ([]Job, error) {
	parsed, err := url.Parse(careerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing career URL: %w", err)
	}
	origin := parsed.Scheme + "://" + parsed.Host
	careerURL = strings.TrimRight(careerURL, "/")

	resp, err := get(ctx, careerURL+"/search-jobs", false)
	if err != nil {
		return nil, fmt.Errorf("fetching career page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrUnsupportedPlatform
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("career page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing career page HTML: %w", err)
	}

	// This container is how TalentBrew pages describe themselves; if it's
	// missing, the selectors below would silently match nothing.
	section := doc.Find("section#search-results")
	if section.Length() == 0 {
		return nil, ErrUnsupportedPlatform
	}
	attr := func(name string) string {
		v, _ := section.Attr(name)
		return v
	}

	totalPages, _ := strconv.Atoi(attr("data-total-pages"))
	if totalPages < 1 {
		totalPages = 1
	}
	if maxPages > 0 && totalPages > maxPages {
		totalPages = maxPages
	}
	ajaxPath := attr("data-ajax-url")
	if ajaxPath == "" {
		totalPages = 1
	}

	// Indexed by page number so results stay in the site's order even
	// though pages are fetched concurrently.
	pages := make([][]Job, totalPages+1)
	pages[1] = parseJobs(doc.Selection, origin)

	params := url.Values{
		"ActiveFacetID":           {attr("data-active-facet-id")},
		"RecordsPerPage":          {attr("data-records-per-page")},
		"TotalContentResults":     {""},
		"Distance":                {attr("data-distance")},
		"RadiusUnitType":          {"0"},
		"Keywords":                {""},
		"Location":                {""},
		"ShowRadius":              {attr("data-show-radius")},
		"IsPagination":            {"True"},
		"CustomFacetName":         {""},
		"FacetTerm":               {""},
		"FacetType":               {attr("data-facet-type")},
		"SearchResultsModuleName": {attr("data-search-results-module-name")},
		"SortCriteria":            {attr("data-sort-criteria")},
		"SortDirection":           {attr("data-sort-direction")},
		"SearchType":              {attr("data-search-type")},
		"PostalCode":              {""},
		"ResultsType":             {attr("data-results-type")},
		"fc":                      {""},
		"fl":                      {""},
		"fcf":                     {""},
		"afc":                     {""},
		"afl":                     {""},
		"afcf":                    {""},
		"TotalContentPages":       {"NaN"},
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentPages)
	for page := 2; page <= totalPages; page++ {
		g.Go(func() error {
			jobs, err := fetchTalentBrewPage(gctx, origin+ajaxPath, params, page, origin)
			if err != nil {
				return fmt.Errorf("page %d: %w", page, err)
			}
			pages[page] = jobs
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var all []Job
	for _, p := range pages {
		all = append(all, p...)
	}
	return all, nil
}

// fetchTalentBrewPage loads one page of results from TalentBrew's AJAX
// endpoint, which returns JSON wrapping an HTML fragment of job listings.
func fetchTalentBrewPage(ctx context.Context, endpoint string, params url.Values, page int, origin string) ([]Job, error) {
	q := make(url.Values, len(params)+1)
	for k, v := range params {
		q[k] = v
	}
	q.Set("CurrentPage", strconv.Itoa(page))

	resp, err := get(ctx, endpoint+"?"+q.Encode(), true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var payload struct {
		Results string `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload.Results))
	if err != nil {
		return nil, fmt.Errorf("parsing results HTML: %w", err)
	}
	return parseJobs(doc.Selection, origin), nil
}

func parseJobs(root *goquery.Selection, origin string) []Job {
	var jobs []Job
	root.Find("li.section29__search-results-li").Each(func(i int, s *goquery.Selection) {
		link := s.Find("a.section29__search-results-link")
		href, _ := link.Attr("href")
		id, _ := link.Attr("data-job-id")

		jobs = append(jobs, Job{
			ID:       id,
			Title:    strings.TrimSpace(link.Find("h2.section29__search-results-job-title").Text()),
			Location: strings.TrimSpace(link.Find("span.section29__result-location").Text()),
			URL:      origin + href,
		})
	})
	return jobs
}

func get(ctx context.Context, rawURL string, ajax bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	// Without a browser-like User-Agent, some sites serve a stripped-down
	// page or block the request outright.
	req.Header.Set("User-Agent", "Mozilla/5.0")
	if ajax {
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	return httpClient.Do(req)
}
