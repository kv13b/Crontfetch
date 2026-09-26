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

// fakeBeeSite mimics the real API: 1-based FirstItem, up to CountItem
// results, the total reported on every in-range page, and one listing
// repeated across a page boundary.
func fakeBeeSite(t *testing.T, total int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			SearchParameters struct {
				FirstItem int `json:"FirstItem"`
				CountItem int `json:"CountItem"`
			} `json:"SearchParameters"`
		}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("data")), &req); err != nil {
			http.Error(w, "bad data", http.StatusBadRequest)
			return
		}

		type loc struct {
			CityName    string `json:"CityName"`
			CountryName string `json:"CountryName"`
		}
		type descriptor struct {
			ID        string `json:"ID"`
			Title     string `json:"PositionTitle"`
			URI       string `json:"PositionURI"`
			Locations []loc  `json:"PositionLocation"`
		}
		type item struct {
			Descriptor descriptor `json:"MatchedObjectDescriptor"`
		}

		items := []item{}
		start := req.SearchParameters.FirstItem - 1
		for i := start; i < start+req.SearchParameters.CountItem && i < total; i++ {
			n := i
			if i == beeSitePageSize { // repeat the last job of page one
				n = beeSitePageSize - 1
			}
			items = append(items, item{descriptor{
				ID:        fmt.Sprint(9000 + n),
				Title:     fmt.Sprintf(" Engineer %d ", n),
				URI:       fmt.Sprintf("https://jobs.example.com/engineer-%d", n),
				Locations: []loc{{"Stuttgart", "Germany"}, {"Stuttgart", "Germany"}, {"Berlin", "Germany"}},
			}})
		}
		json.NewEncoder(w).Encode(map[string]any{"SearchResult": map[string]any{
			"SearchResultCountAll": total,
			"SearchResultItems":    items,
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchBeeSiteJobs(t *testing.T) {
	const total = 250
	srv := fakeBeeSite(t, total)

	jobs, err := FetchBeeSiteJobs(context.Background(), srv.URL+"/", 100)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(jobs) != total-1 { // one repeated listing removed
		t.Fatalf("got %d jobs, want %d", len(jobs), total-1)
	}
	first := jobs[0]
	want := Job{ID: "9000", Title: "Engineer 0", Location: "Stuttgart, Germany; Berlin, Germany", URL: "https://jobs.example.com/engineer-0"}
	if first != want {
		t.Errorf("first job = %+v, want %+v", first, want)
	}
	if last := jobs[len(jobs)-1]; last.ID != "9249" {
		t.Errorf("last job ID = %s, want 9249 (page order not preserved)", last.ID)
	}

	if capped, err := FetchBeeSiteJobs(context.Background(), srv.URL, 1); err != nil || len(capped) != beeSitePageSize {
		t.Errorf("maxPages=1 gave %d jobs, err %v; want %d", len(capped), err, beeSitePageSize)
	}
}

func TestFetchBeeSiteJobs_Errors(t *testing.T) {
	srv := fakeBeeSite(t, 5)
	notBeeSite := servePage(t, `{"hello":"world"}`)

	tests := []struct {
		name string
		base string
		want error
	}{
		{"no address", "", ErrMissingBoard},
		{"not a url", "://nope", ErrBoardNotFound},
		{"path that 404s", srv.URL + "/wrong-prefix", ErrBoardNotFound},
		{"json that isn't a BeeSite API", notBeeSite.URL, ErrBoardNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := FetchBeeSiteJobs(context.Background(), tt.base, 1); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestScanPage_BeeSite(t *testing.T) {
	tests := []struct {
		name string
		html string
		want Detection
	}{
		{"mercedes-style page config",
			`window.__NUXT__.config={public:{gjbAddress:"https://jobs.api.mercedes-benz.com",gjbAddressIntranet:"https://x.beesite.de"}}`,
			Detection{PlatformBeeSite, "https://jobs.api.mercedes-benz.com"}},
		{"trailing slash trimmed", `gjbAddress: 'https://jobs.api.acme.com/'`, Detection{PlatformBeeSite, "https://jobs.api.acme.com"}},
		{"intranet key alone isn't the public API", `gjbAddressIntranet:"https://x.beesite.de"`, Detection{}},
		{"non-https address ignored", `gjbAddress:"http://internal.local"`, Detection{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanPage(tt.html); got != tt.want {
				t.Errorf("scanPage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Live check against the real Mercedes-Benz career site.
func TestFetchBeeSiteJobs_Live(t *testing.T) {
	url := "https://jobs.mercedes-benz.com/"

	if got := Detect(context.Background(), url); got != (Detection{PlatformBeeSite, "https://jobs.api.mercedes-benz.com"}) {
		t.Fatalf("Detect() = %+v", got)
	}

	jobs, err := FetchJobs(context.Background(), url, "", "", 2)
	if err != nil {
		t.Fatalf("FetchJobs() error = %v", err)
	}
	if len(jobs) != 2*beeSitePageSize {
		t.Errorf("got %d jobs, want %d (2 pages)", len(jobs), 2*beeSitePageSize)
	}
	for _, j := range jobs {
		if j.ID == "" || j.Title == "" || j.URL == "" {
			t.Fatalf("job missing fields: %+v", j)
		}
	}
	t.Logf("first: %+v", jobs[0])
}
