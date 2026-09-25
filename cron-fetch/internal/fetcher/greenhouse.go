package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// greenhouseAPIBase is a variable so tests can point it at a local server.
var greenhouseAPIBase = "https://boards-api.greenhouse.io/v1/boards"

type greenhouseResponse struct {
	Jobs []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		Location    struct {
			Name string `json:"name"`
		} `json:"location"`
		Metadata []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"metadata"`
	} `json:"jobs"`
}

// FetchGreenhouseJobs reads every listing on a company's Greenhouse job
// board through Greenhouse's public JSON API (one request, no pagination).
func FetchGreenhouseJobs(ctx context.Context, board string) ([]Job, error) {
	if board == "" {
		return nil, ErrMissingBoard
	}

	resp, err := get(ctx, greenhouseAPIBase+"/"+url.PathEscape(board)+"/jobs", false)
	if err != nil {
		return nil, fmt.Errorf("fetching greenhouse board: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrBoardNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("greenhouse returned status %d", resp.StatusCode)
	}

	var payload greenhouseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding greenhouse response: %w", err)
	}

	jobs := make([]Job, 0, len(payload.Jobs))
	for _, j := range payload.Jobs {
		names := []string{j.Location.Name}
		for _, md := range j.Metadata {
			// Companies often keep the real city in a custom "...Location"
			// field and leave the main location as just "Hybrid" or "Remote".
			if strings.Contains(strings.ToLower(md.Name), "location") {
				names = append(names, stringsFromValue(md.Value)...)
			}
		}

		jobs = append(jobs, Job{
			ID:       strconv.FormatInt(j.ID, 10),
			Title:    strings.TrimSpace(j.Title),
			Location: joinUnique(names),
			URL:      j.AbsoluteURL,
		})
	}
	return jobs, nil
}

// stringsFromValue reads a Greenhouse metadata value, which may be a string,
// a list of strings, or null.
func stringsFromValue(v any) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []any:
		var out []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func joinUnique(values []string) string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		key := strings.ToLower(v)
		if v == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return strings.Join(out, "; ")
}
