package fetcher

// Fetchers for hosted job boards that publish a public JSON API. Each takes
// the company's board name (its slug on that platform) and returns Jobs.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// API locations are variables so tests can point them at a local server.
var (
	leverAPIBase           = "https://api.lever.co/v0/postings"
	ashbyAPIBase           = "https://api.ashbyhq.com/posting-api/job-board"
	smartRecruitersAPIBase = "https://api.smartrecruiters.com/v1/companies"
	workableAPIBase        = "https://apply.workable.com/api/v1/widget/accounts"
	recruiteeAPIURL        = func(board string) string { return "https://" + board + ".recruitee.com/api/offers/" }
)

// smartRecruitersPageSize is SmartRecruiters' maximum; larger limits are
// silently reduced to it.
const smartRecruitersPageSize = 100

// getBoardJSON GETs a board's API URL and decodes the JSON into v. An
// unknown board (HTTP 404) becomes ErrBoardNotFound.
func getBoardJSON(ctx context.Context, platform, board, apiURL string, v any) error {
	if board == "" {
		return ErrMissingBoard
	}

	resp, err := get(ctx, apiURL, false)
	if err != nil {
		return fmt.Errorf("fetching %s board: %w", platform, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrBoardNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%s returned status %d", platform, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding %s response: %w", platform, err)
	}
	return nil
}

// FetchLeverJobs reads every posting on a company's Lever board.
func FetchLeverJobs(ctx context.Context, board string) ([]Job, error) {
	var postings []struct {
		ID         string `json:"id"`
		Text       string `json:"text"`
		HostedURL  string `json:"hostedUrl"`
		Categories struct {
			Location     string   `json:"location"`
			AllLocations []string `json:"allLocations"`
		} `json:"categories"`
	}
	if err := getBoardJSON(ctx, "lever", board, leverAPIBase+"/"+url.PathEscape(board)+"?mode=json", &postings); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(postings))
	for _, p := range postings {
		locations := p.Categories.AllLocations
		if len(locations) == 0 {
			locations = []string{p.Categories.Location}
		}
		jobs = append(jobs, Job{
			ID:       p.ID,
			Title:    strings.TrimSpace(p.Text),
			Location: joinUnique(locations),
			URL:      p.HostedURL,
		})
	}
	return jobs, nil
}

// FetchAshbyJobs reads every listed job on a company's Ashby board.
func FetchAshbyJobs(ctx context.Context, board string) ([]Job, error) {
	var payload struct {
		Jobs []struct {
			ID                 string `json:"id"`
			Title              string `json:"title"`
			Location           string `json:"location"`
			SecondaryLocations []struct {
				Location string `json:"location"`
			} `json:"secondaryLocations"`
			IsRemote bool   `json:"isRemote"`
			IsListed *bool  `json:"isListed"`
			JobURL   string `json:"jobUrl"`
		} `json:"jobs"`
	}
	if err := getBoardJSON(ctx, "ashby", board, ashbyAPIBase+"/"+url.PathEscape(board), &payload); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(payload.Jobs))
	for _, j := range payload.Jobs {
		if j.IsListed != nil && !*j.IsListed {
			continue
		}
		locations := []string{j.Location}
		for _, s := range j.SecondaryLocations {
			locations = append(locations, s.Location)
		}
		if j.IsRemote {
			locations = append(locations, "Remote")
		}
		jobs = append(jobs, Job{
			ID:       j.ID,
			Title:    strings.TrimSpace(j.Title),
			Location: joinUnique(locations),
			URL:      j.JobURL,
		})
	}
	return jobs, nil
}

type smartRecruitersPage struct {
	TotalFound int `json:"totalFound"`
	Content    []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Location struct {
			FullLocation string `json:"fullLocation"`
			Remote       bool   `json:"remote"`
		} `json:"location"`
	} `json:"content"`
}

// FetchSmartRecruitersJobs reads a company's SmartRecruiters postings, 100
// per page. maxPages caps how many pages are read.
//
// SmartRecruiters answers 200 with zero results for a company that doesn't
// exist, so an empty board is reported as ErrBoardNotFound.
func FetchSmartRecruitersJobs(ctx context.Context, board string, maxPages int) ([]Job, error) {
	pageURL := func(offset int) string {
		return smartRecruitersAPIBase + "/" + url.PathEscape(board) + "/postings?limit=" +
			strconv.Itoa(smartRecruitersPageSize) + "&offset=" + strconv.Itoa(offset)
	}

	var first smartRecruitersPage
	if err := getBoardJSON(ctx, "smartrecruiters", board, pageURL(0), &first); err != nil {
		return nil, err
	}
	if first.TotalFound == 0 {
		return nil, ErrBoardNotFound
	}

	totalPages := (first.TotalFound + smartRecruitersPageSize - 1) / smartRecruitersPageSize
	if maxPages > 0 && totalPages > maxPages {
		totalPages = maxPages
	}

	// Indexed by page so results stay in order despite concurrent fetching.
	pages := make([]smartRecruitersPage, totalPages)
	pages[0] = first

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentPages)
	for page := 1; page < totalPages; page++ {
		g.Go(func() error {
			var p smartRecruitersPage
			if err := getBoardJSON(gctx, "smartrecruiters", board, pageURL(page*smartRecruitersPageSize), &p); err != nil {
				return fmt.Errorf("page %d: %w", page+1, err)
			}
			pages[page] = p
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Postings added mid-scan can shift the offsets and repeat a listing.
	seen := make(map[string]bool)
	var jobs []Job
	for _, p := range pages {
		for _, c := range p.Content {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true

			locations := []string{c.Location.FullLocation}
			if c.Location.Remote {
				locations = append(locations, "Remote")
			}
			jobs = append(jobs, Job{
				ID:       c.ID,
				Title:    strings.TrimSpace(c.Name),
				Location: joinUnique(locations),
				URL:      "https://jobs.smartrecruiters.com/" + board + "/" + c.ID,
			})
		}
	}
	return jobs, nil
}

// FetchWorkableJobs reads every job on a company's Workable account through
// its public widget API.
func FetchWorkableJobs(ctx context.Context, board string) ([]Job, error) {
	var payload struct {
		Jobs []struct {
			Title         string `json:"title"`
			Shortcode     string `json:"shortcode"`
			URL           string `json:"url"`
			Telecommuting bool   `json:"telecommuting"`
			City          string `json:"city"`
			State         string `json:"state"`
			Country       string `json:"country"`
			Locations     []struct {
				City    string `json:"city"`
				Region  string `json:"region"`
				Country string `json:"country"`
			} `json:"locations"`
		} `json:"jobs"`
	}
	if err := getBoardJSON(ctx, "workable", board, workableAPIBase+"/"+url.PathEscape(board), &payload); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(payload.Jobs))
	for _, j := range payload.Jobs {
		var locations []string
		for _, l := range j.Locations {
			locations = append(locations, joinParts(l.City, l.Region, l.Country))
		}
		if len(locations) == 0 {
			locations = append(locations, joinParts(j.City, j.State, j.Country))
		}
		if j.Telecommuting {
			locations = append(locations, "Remote")
		}
		jobs = append(jobs, Job{
			ID:       j.Shortcode,
			Title:    strings.TrimSpace(j.Title),
			Location: joinUnique(locations),
			URL:      j.URL,
		})
	}
	return jobs, nil
}

// FetchRecruiteeJobs reads every published offer on a company's Recruitee
// board (board is the subdomain in <board>.recruitee.com).
func FetchRecruiteeJobs(ctx context.Context, board string) ([]Job, error) {
	var payload struct {
		Offers []struct {
			ID         int64  `json:"id"`
			Title      string `json:"title"`
			Location   string `json:"location"`
			City       string `json:"city"`
			Country    string `json:"country"`
			Remote     bool   `json:"remote"`
			Status     string `json:"status"`
			CareersURL string `json:"careers_url"`
		} `json:"offers"`
	}
	if err := getBoardJSON(ctx, "recruitee", board, recruiteeAPIURL(url.PathEscape(board)), &payload); err != nil {
		return nil, err
	}

	jobs := make([]Job, 0, len(payload.Offers))
	for _, o := range payload.Offers {
		if o.Status != "" && o.Status != "published" {
			continue
		}
		location := o.Location
		if location == "" {
			location = joinParts(o.City, o.Country)
		}
		locations := []string{location}
		if o.Remote {
			locations = append(locations, "Remote")
		}
		jobs = append(jobs, Job{
			ID:       strconv.FormatInt(o.ID, 10),
			Title:    strings.TrimSpace(o.Title),
			Location: joinUnique(locations),
			URL:      o.CareersURL,
		})
	}
	return jobs, nil
}

// joinParts joins the non-empty parts with ", ".
func joinParts(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}
