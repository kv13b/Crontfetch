// Package fetcher scrapes job listings from company career pages.
package fetcher

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Job is a single job listing pulled off a career page.
type Job struct {
	ID       string
	Title    string
	Location string
	URL      string
}

// FetchTalentBrewJobs scrapes the first page of search results (~15 jobs)
// from a career page built on the TalentBrew/Radancy platform (identified
// by markup like Palo Alto Networks' career site). careerURL is the base
// career page, e.g. "https://jobs.paloaltonetworks.com/en" — the search
// results page and job links are derived from it, so this works for any
// company on the same platform, not just one hardcoded site.
//
// It's scoped to page one for now; paginating through all listings is a
// follow-up.
func FetchTalentBrewJobs(careerURL string) ([]Job, error) {
	base, err := url.Parse(careerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing career URL: %w", err)
	}
	origin := base.Scheme + "://" + base.Host

	searchURL := strings.TrimRight(careerURL, "/") + "/search-jobs"

	req, err := http.NewRequest(http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	// Without a browser-like User-Agent, some sites serve a stripped-down
	// page or block the request outright.
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching career page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("career page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parsing career page HTML: %w", err)
	}

	var jobs []Job
	doc.Find("li.section29__search-results-li").Each(func(i int, s *goquery.Selection) {
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

	return jobs, nil
}
