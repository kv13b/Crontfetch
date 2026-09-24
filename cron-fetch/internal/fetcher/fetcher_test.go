package fetcher

import (
	"context"
	"errors"
	"testing"
)

// The tests below hit real career sites. They're here to manually verify
// the scraper still matches the sites' current markup, not for routine CI
// (a layout change on their end would break them).

func TestFetchTalentBrewJobs(t *testing.T) {
	jobs, err := FetchTalentBrewJobs(context.Background(), "https://jobs.paloaltonetworks.com/en", 3)
	if err != nil {
		t.Fatalf("FetchTalentBrewJobs() error = %v", err)
	}
	if len(jobs) <= 15 {
		t.Fatalf("expected more than one page of jobs (15), got %d", len(jobs))
	}

	seen := make(map[string]bool)
	for _, j := range jobs {
		if j.ID == "" || j.Title == "" || j.URL == "" {
			t.Errorf("job missing fields: %+v", j)
		}
		if seen[j.ID] {
			t.Errorf("duplicate job ID %s: pages overlapped", j.ID)
		}
		seen[j.ID] = true
	}
	t.Logf("fetched %d unique jobs across 3 pages", len(jobs))
}

func TestFetchTalentBrewJobs_UnsupportedSite(t *testing.T) {
	_, err := FetchTalentBrewJobs(context.Background(), "https://example.com/careers", 1)
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("error = %v, want ErrUnsupportedPlatform", err)
	}
}
