package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseWorkdayURL(t *testing.T) {
	tests := []struct {
		url  string
		want workdaySite
		ok   bool
	}{
		{"https://nvidia.wd5.myworkdayjobs.com/NVIDIAExternalCareerSite", workdaySite{"nvidia.wd5.myworkdayjobs.com", "nvidia", "NVIDIAExternalCareerSite"}, true},
		{"https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/", workdaySite{"nvidia.wd5.myworkdayjobs.com", "nvidia", "NVIDIAExternalCareerSite"}, true},
		{"https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/US-CA/Some-Job_JR1", workdaySite{"nvidia.wd5.myworkdayjobs.com", "nvidia", "NVIDIAExternalCareerSite"}, true},
		{"https://nvidia.wd5.myworkdayjobs.com/", workdaySite{}, false},
		{"https://example.com/NVIDIAExternalCareerSite", workdaySite{}, false},
		{"not a url", workdaySite{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got, ok := parseWorkdayURL(tt.url)
			if ok != tt.ok || got != tt.want {
				t.Errorf("parseWorkdayURL() = (%+v, %v), want (%+v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDetectPlatform_Workday(t *testing.T) {
	platform, board := DetectPlatform("https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite")
	if platform != PlatformWorkday || board != "" {
		t.Errorf("DetectPlatform() = (%q, %q), want (%q, \"\")", platform, board, PlatformWorkday)
	}
}

// The fake server mimics Workday's quirks: 45 jobs at 20 per page, the total
// only reported on the first page, and one listing repeated across a page
// boundary (as happens when a job is posted mid-scan).
func TestFetchWorkdayJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if r.Method != http.MethodPost || req.Limit != workdayPageSize {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		total := 45
		type posting struct {
			Title         string   `json:"title"`
			ExternalPath  string   `json:"externalPath"`
			LocationsText string   `json:"locationsText"`
			BulletFields  []string `json:"bulletFields"`
		}
		var postings []posting
		for i := req.Offset; i < req.Offset+req.Limit && i < total; i++ {
			n := i
			if i == 20 { // repeat job 19 at the start of page two
				n = 19
			}
			postings = append(postings, posting{
				Title:         fmt.Sprintf(" Engineer %d ", n),
				ExternalPath:  fmt.Sprintf("/job/City/Engineer-%d_JR%d", n, n),
				LocationsText: "India, Bangalore",
				BulletFields:  []string{fmt.Sprintf("JR%d", n)},
			})
		}

		reported := 0
		if req.Offset == 0 {
			reported = total
		}
		json.NewEncoder(w).Encode(map[string]any{"total": reported, "jobPostings": postings})
	}))
	defer srv.Close()

	old := workdayAPIURL
	workdayAPIURL = func(workdaySite) string { return srv.URL }
	t.Cleanup(func() { workdayAPIURL = old })

	jobs, err := FetchWorkdayJobs(context.Background(), "https://acme.wd1.myworkdayjobs.com/en-US/Careers", 100)
	if err != nil {
		t.Fatalf("FetchWorkdayJobs() error = %v", err)
	}
	if len(jobs) != 44 { // 45 slots minus the one repeated listing
		t.Fatalf("got %d jobs, want 44", len(jobs))
	}
	first := jobs[0]
	if first.ID != "JR0" || first.Title != "Engineer 0" || first.Location != "India, Bangalore" ||
		first.URL != "https://acme.wd1.myworkdayjobs.com/Careers/job/City/Engineer-0_JR0" {
		t.Errorf("first job = %+v", first)
	}
	if jobs[43].ID != "JR44" {
		t.Errorf("last job ID = %s, want JR44 (page order not preserved)", jobs[43].ID)
	}

	capped, err := FetchWorkdayJobs(context.Background(), "https://acme.wd1.myworkdayjobs.com/Careers", 1)
	if err != nil || len(capped) != 20 {
		t.Errorf("maxPages=1 gave %d jobs, err %v; want 20", len(capped), err)
	}

	if _, err := FetchWorkdayJobs(context.Background(), "https://example.com/careers", 1); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("non-workday URL error = %v, want ErrUnsupportedPlatform", err)
	}
}

// Live checks against a real Workday tenant.
func TestFetchWorkdayJobs_Live(t *testing.T) {
	jobs, err := FetchJobs(context.Background(), "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite", "", "", 3)
	if err != nil {
		t.Fatalf("FetchJobs() error = %v", err)
	}
	if len(jobs) != 60 {
		t.Errorf("expected 3 pages x 20 = 60 jobs, got %d", len(jobs))
	}
	t.Logf("first: %+v", jobs[0])

	_, err = FetchJobs(context.Background(), "https://nvidia.wd5.myworkdayjobs.com/NoSuchSite", "", "", 1)
	if !errors.Is(err, ErrWorkdaySiteNotFound) {
		t.Errorf("wrong site error = %v, want ErrWorkdaySiteNotFound", err)
	}
}
