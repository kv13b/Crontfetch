package fetcher

import "testing"

// TestFetchTalentBrewJobs hits the real career site. It's here to manually
// verify the scraper still matches the site's current markup, not as part
// of routine CI (a layout change on their end would break this).
func TestFetchTalentBrewJobs(t *testing.T) {
	jobs, err := FetchTalentBrewJobs("https://jobs.paloaltonetworks.com/en")
	if err != nil {
		t.Fatalf("FetchTalentBrewJobs() error = %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("FetchTalentBrewJobs() returned no jobs")
	}

	for _, j := range jobs {
		t.Logf("%s | %s | %s | %s", j.ID, j.Title, j.Location, j.URL)
		if j.ID == "" || j.Title == "" || j.URL == "" {
			t.Errorf("job missing fields: %+v", j)
		}
	}
}
