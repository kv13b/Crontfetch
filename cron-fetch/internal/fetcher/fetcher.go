// Package fetcher scrapes job listings from company career pages.
package fetcher

import (
	"fmt"
	"net/http"
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

const paloAltoSearchURL = "https://jobs.paloaltonetworks.com/en/search-jobs"

// FetchPaloAltoJobs scrapes the first page of Palo Alto Networks' career
// site search results (~15 jobs). It's scoped to page one for now;
// paginating through all listings is a follow-up.
func FetchPaloAltoJobs() ([]Job, error) {
	req, err := http.NewRequest(http.MethodGet, paloAltoSearchURL, nil)
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
			URL:      "https://jobs.paloaltonetworks.com" + href,
		})
	})

	return jobs, nil
}
