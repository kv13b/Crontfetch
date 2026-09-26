package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/sync/errgroup"
)

// beeSitePageSize is well under the API's limit (500 works, 1000 errors).
const beeSitePageSize = 100

// beeSiteAPIRe finds a BeeSite front-end's job API address in its page
// config, e.g. gjbAddress:"https://jobs.api.mercedes-benz.com". It must not
// match the neighbouring gjbAddressIntranet key.
var beeSiteAPIRe = regexp.MustCompile(`gjbAddress\s*:\s*["'](https://[^"']+)["']`)

type beeSitePage struct {
	// A pointer, so a response that isn't a BeeSite API at all can be told
	// apart from a genuine search with zero results.
	SearchResult *struct {
		CountAll int `json:"SearchResultCountAll"`
		Items    []struct {
			Descriptor struct {
				ID         string `json:"ID"`
				PositionID string `json:"PositionID"`
				Title      string `json:"PositionTitle"`
				URI        string `json:"PositionURI"`
				Locations  []struct {
					CityName    string `json:"CityName"`
					CountryName string `json:"CountryName"`
				} `json:"PositionLocation"`
			} `json:"MatchedObjectDescriptor"`
		} `json:"SearchResultItems"`
	} `json:"SearchResult"`
}

// beeSiteSearchURL builds the search request for one page. FirstItem is 1-based.
func beeSiteSearchURL(apiBase string, page int) string {
	data, _ := json.Marshal(map[string]any{
		"LanguageCode": "EN",
		"SearchParameters": map[string]any{
			"FirstItem": page*beeSitePageSize + 1,
			"CountItem": beeSitePageSize,
		},
		"SearchCriteria": []any{},
	})
	return strings.TrimRight(apiBase, "/") + "/search/?data=" + url.QueryEscape(string(data))
}

// FetchBeeSiteJobs reads every listing from a BeeSite job API (used, for
// example, by jobs.mercedes-benz.com). apiBase is the API's address, such as
// "https://jobs.api.mercedes-benz.com". maxPages caps how many pages (100
// jobs each) are read.
func FetchBeeSiteJobs(ctx context.Context, apiBase string, maxPages int) ([]Job, error) {
	if u, err := url.Parse(apiBase); apiBase != "" && (err != nil || u.Host == "") {
		return nil, ErrBoardNotFound
	}

	var first beeSitePage
	if err := getBoardJSON(ctx, "beesite", apiBase, beeSiteSearchURL(apiBase, 0), &first); err != nil {
		return nil, err
	}
	if first.SearchResult == nil {
		return nil, ErrBoardNotFound
	}

	totalPages := (first.SearchResult.CountAll + beeSitePageSize - 1) / beeSitePageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if maxPages > 0 && totalPages > maxPages {
		totalPages = maxPages
	}

	// Indexed by page so results stay in order despite concurrent fetching.
	pages := make([]beeSitePage, totalPages)
	pages[0] = first

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrentPages)
	for page := 1; page < totalPages; page++ {
		g.Go(func() error {
			var p beeSitePage
			if err := getBoardJSON(gctx, "beesite", apiBase, beeSiteSearchURL(apiBase, page), &p); err != nil {
				return fmt.Errorf("page %d: %w", page+1, err)
			}
			pages[page] = p
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Jobs published mid-scan can shift the offsets and repeat a listing.
	seen := make(map[string]bool)
	var jobs []Job
	for _, p := range pages {
		if p.SearchResult == nil {
			continue
		}
		for _, item := range p.SearchResult.Items {
			d := item.Descriptor
			id := d.ID
			if id == "" {
				id = d.PositionID
			}
			if seen[id] {
				continue
			}
			seen[id] = true

			var locations []string
			for _, l := range d.Locations {
				locations = append(locations, joinParts(l.CityName, l.CountryName))
			}
			jobs = append(jobs, Job{
				ID:       id,
				Title:    strings.TrimSpace(d.Title),
				Location: joinUnique(locations),
				URL:      d.URI,
			})
		}
	}
	return jobs, nil
}
