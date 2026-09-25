package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		url          string
		wantPlatform string
		wantBoard    string
	}{
		{"https://boards.greenhouse.io/cloudflare", PlatformGreenhouse, "cloudflare"},
		{"https://boards.greenhouse.io/cloudflare/jobs/123", PlatformGreenhouse, "cloudflare"},
		{"https://job-boards.greenhouse.io/acme/", PlatformGreenhouse, "acme"},
		{"https://boards.greenhouse.io/embed/job_board?for=acme", PlatformGreenhouse, "acme"},
		{"https://www.cloudflare.com/careers/jobs/", "", ""},
		{"https://jobs.paloaltonetworks.com/en", "", ""},
		{"not a url", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			platform, board := DetectPlatform(tt.url)
			if platform != tt.wantPlatform || board != tt.wantBoard {
				t.Errorf("DetectPlatform() = (%q, %q), want (%q, %q)", platform, board, tt.wantPlatform, tt.wantBoard)
			}
		})
	}
}

func TestFetchGreenhouseJobs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/acme/jobs":
			w.Write([]byte(`{"jobs":[
				{"id":1,"title":" Backend Engineer ","absolute_url":"https://example.com/1","location":{"name":"Hybrid"},
				 "metadata":[
				   {"name":"Cost Center","value":"5150"},
				   {"name":"Job Posting Location","value":["Bangalore, India","Hybrid"]},
				   {"name":"Career Site Department","value":null}]},
				{"id":2,"title":"Designer","absolute_url":"https://example.com/2","location":{"name":"Remote"},"metadata":null}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	old := greenhouseAPIBase
	greenhouseAPIBase = srv.URL
	t.Cleanup(func() { greenhouseAPIBase = old })

	got, err := FetchGreenhouseJobs(context.Background(), "acme")
	if err != nil {
		t.Fatalf("FetchGreenhouseJobs() error = %v", err)
	}
	want := []Job{
		{ID: "1", Title: "Backend Engineer", Location: "Hybrid; Bangalore, India", URL: "https://example.com/1"},
		{ID: "2", Title: "Designer", Location: "Remote", URL: "https://example.com/2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FetchGreenhouseJobs() = %+v, want %+v", got, want)
	}

	if _, err := FetchGreenhouseJobs(context.Background(), "nope"); !errors.Is(err, ErrBoardNotFound) {
		t.Errorf("unknown board error = %v, want ErrBoardNotFound", err)
	}
	if _, err := FetchGreenhouseJobs(context.Background(), ""); !errors.Is(err, ErrMissingBoard) {
		t.Errorf("empty board error = %v, want ErrMissingBoard", err)
	}
}

// Live check against the real Greenhouse API, like the TalentBrew tests.
func TestFetchGreenhouseJobs_Live(t *testing.T) {
	jobs, err := FetchJobs(context.Background(), "https://www.cloudflare.com/careers/jobs/", PlatformGreenhouse, "cloudflare", 1)
	if err != nil {
		t.Fatalf("FetchJobs() error = %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("expected Cloudflare jobs, got none")
	}
	t.Logf("fetched %d jobs; first: %+v", len(jobs), jobs[0])
}
