package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

// fakeBoardServer serves canned JSON at exactly one path and 404s the rest.
func fakeBoardServer(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// override swaps a package variable for the test and restores it afterwards.
func override[T any](t *testing.T, target *T, value T) {
	t.Helper()
	old := *target
	*target = value
	t.Cleanup(func() { *target = old })
}

func TestFetchLeverJobs(t *testing.T) {
	srv := fakeBoardServer(t, "/acme", `[
		{"id":"a1","text":" Android Engineer ","hostedUrl":"https://jobs.lever.co/acme/a1",
		 "categories":{"location":"London","allLocations":["London","Stockholm"]},
		 "lists":[{"text":"What You'll Do","content":"<li>x</li>"}]},
		{"id":"a2","text":"Designer","hostedUrl":"https://jobs.lever.co/acme/a2","categories":{"location":"Remote"}}
	]`)
	override(t, &leverAPIBase, srv.URL)

	got, err := FetchLeverJobs(context.Background(), "acme")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []Job{
		{ID: "a1", Title: "Android Engineer", Location: "London; Stockholm", URL: "https://jobs.lever.co/acme/a1"},
		{ID: "a2", Title: "Designer", Location: "Remote", URL: "https://jobs.lever.co/acme/a2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchAshbyJobs(t *testing.T) {
	srv := fakeBoardServer(t, "/acme", `{"jobs":[
		{"id":"j1","title":"Fullstack Engineer","location":"Europe","isRemote":true,"isListed":true,
		 "secondaryLocations":[{"location":"New York, New York"}],"jobUrl":"https://jobs.ashbyhq.com/acme/j1"},
		{"id":"j2","title":"Hidden","location":"Paris","isListed":false,"jobUrl":"https://jobs.ashbyhq.com/acme/j2"},
		{"id":"j3","title":"No listed flag","location":"Berlin","jobUrl":"https://jobs.ashbyhq.com/acme/j3"}
	]}`)
	override(t, &ashbyAPIBase, srv.URL)

	got, err := FetchAshbyJobs(context.Background(), "acme")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []Job{
		{ID: "j1", Title: "Fullstack Engineer", Location: "Europe; New York, New York; Remote", URL: "https://jobs.ashbyhq.com/acme/j1"},
		{ID: "j3", Title: "No listed flag", Location: "Berlin", URL: "https://jobs.ashbyhq.com/acme/j3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchWorkableJobs(t *testing.T) {
	srv := fakeBoardServer(t, "/acme", `{"name":"Acme","jobs":[
		{"title":"Senior Engineer","shortcode":"F4C0","url":"https://apply.workable.com/j/F4C0","telecommuting":true,
		 "locations":[{"city":"Paris","region":"Île-de-France","country":"France"}]},
		{"title":"Support","shortcode":"AA11","url":"https://apply.workable.com/j/AA11","city":"Austin","state":"Texas","country":"United States"}
	]}`)
	override(t, &workableAPIBase, srv.URL)

	got, err := FetchWorkableJobs(context.Background(), "acme")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []Job{
		{ID: "F4C0", Title: "Senior Engineer", Location: "Paris, Île-de-France, France; Remote", URL: "https://apply.workable.com/j/F4C0"},
		{ID: "AA11", Title: "Support", Location: "Austin, Texas, United States", URL: "https://apply.workable.com/j/AA11"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchRecruiteeJobs(t *testing.T) {
	srv := fakeBoardServer(t, "/acme/api/offers/", `{"offers":[
		{"id":39979,"title":"(Senior) Legal Counsel","location":"Amsterdam, Noord-Holland, Netherlands","status":"published","remote":false,"careers_url":"https://careers.acme.com/o/legal"},
		{"id":2,"title":"Draft role","location":"Paris","status":"draft","careers_url":"https://careers.acme.com/o/draft"},
		{"id":3,"title":"Engineer","city":"Berlin","country":"Germany","remote":true,"careers_url":"https://careers.acme.com/o/eng"}
	]}`)
	override(t, &recruiteeAPIURL, func(board string) string { return srv.URL + "/" + board + "/api/offers/" })

	got, err := FetchRecruiteeJobs(context.Background(), "acme")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := []Job{
		{ID: "39979", Title: "(Senior) Legal Counsel", Location: "Amsterdam, Noord-Holland, Netherlands", URL: "https://careers.acme.com/o/legal"},
		{ID: "3", Title: "Engineer", Location: "Berlin, Germany; Remote", URL: "https://careers.acme.com/o/eng"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestFetchSmartRecruitersJobs(t *testing.T) {
	const total = 250
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/acme/postings":
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
			type location struct {
				FullLocation string `json:"fullLocation"`
				Remote       bool   `json:"remote"`
			}
			type posting struct {
				ID       string   `json:"id"`
				Name     string   `json:"name"`
				Location location `json:"location"`
			}
			var content []posting
			for i := offset; i < offset+smartRecruitersPageSize && i < total; i++ {
				n := i
				if i == 100 { // repeat a posting across a page boundary
					n = 99
				}
				content = append(content, posting{ID: strconv.Itoa(1000 + n), Name: fmt.Sprintf(" Role %d ", n), Location: location{FullLocation: "Pleasanton, CA, United States", Remote: n%2 == 0}})
			}
			json.NewEncoder(w).Encode(map[string]any{"totalFound": total, "content": content})
		case "/empty/postings":
			w.Write([]byte(`{"offset":0,"limit":100,"totalFound":0,"content":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	override(t, &smartRecruitersAPIBase, srv.URL)

	jobs, err := FetchSmartRecruitersJobs(context.Background(), "acme", 100)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(jobs) != total-1 { // one repeated posting removed
		t.Fatalf("got %d jobs, want %d", len(jobs), total-1)
	}
	first := jobs[0]
	if first.ID != "1000" || first.Title != "Role 0" || first.Location != "Pleasanton, CA, United States; Remote" ||
		first.URL != "https://jobs.smartrecruiters.com/acme/1000" {
		t.Errorf("first job = %+v", first)
	}
	if last := jobs[len(jobs)-1]; last.ID != "1249" {
		t.Errorf("last job ID = %s, want 1249 (page order not preserved)", last.ID)
	}

	if capped, err := FetchSmartRecruitersJobs(context.Background(), "acme", 1); err != nil || len(capped) != smartRecruitersPageSize {
		t.Errorf("maxPages=1 gave %d jobs, err %v; want %d", len(capped), err, smartRecruitersPageSize)
	}
	if _, err := FetchSmartRecruitersJobs(context.Background(), "empty", 1); !errors.Is(err, ErrBoardNotFound) {
		t.Errorf("zero-result company error = %v, want ErrBoardNotFound", err)
	}
}

func TestBoardFetchers_MissingAndUnknownBoard(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	override(t, &leverAPIBase, srv.URL)
	override(t, &ashbyAPIBase, srv.URL)
	override(t, &smartRecruitersAPIBase, srv.URL)
	override(t, &workableAPIBase, srv.URL)
	override(t, &recruiteeAPIURL, func(board string) string { return srv.URL + "/" + board })

	ctx := context.Background()
	fetchers := map[string]func(board string) error{
		"lever":           func(b string) error { _, err := FetchLeverJobs(ctx, b); return err },
		"ashby":           func(b string) error { _, err := FetchAshbyJobs(ctx, b); return err },
		"smartrecruiters": func(b string) error { _, err := FetchSmartRecruitersJobs(ctx, b, 1); return err },
		"workable":        func(b string) error { _, err := FetchWorkableJobs(ctx, b); return err },
		"recruitee":       func(b string) error { _, err := FetchRecruiteeJobs(ctx, b); return err },
	}
	for name, fetch := range fetchers {
		t.Run(name, func(t *testing.T) {
			if err := fetch(""); !errors.Is(err, ErrMissingBoard) {
				t.Errorf("empty board error = %v, want ErrMissingBoard", err)
			}
			if err := fetch("nope"); !errors.Is(err, ErrBoardNotFound) {
				t.Errorf("unknown board error = %v, want ErrBoardNotFound", err)
			}
		})
	}
}

// Live checks against real companies on each platform. Like the other live
// tests they can break if a company leaves the platform.
func TestJSONBoards_Live(t *testing.T) {
	tests := []struct {
		platform, board string
		maxPages        int
	}{
		{PlatformLever, "spotify", 1},
		{PlatformAshby, "linear", 1},
		{PlatformSmartRecruiters, "BoschGroup", 2},
		{PlatformWorkable, "huggingface", 1},
		{PlatformRecruitee, "bunq", 1},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			jobs, err := FetchJobs(context.Background(), "https://example.com/careers", tt.platform, tt.board, tt.maxPages)
			if err != nil {
				t.Fatalf("FetchJobs() error = %v", err)
			}
			if len(jobs) == 0 {
				t.Fatal("no jobs returned")
			}
			for _, j := range jobs {
				if j.ID == "" || j.Title == "" || j.URL == "" {
					t.Fatalf("job missing fields: %+v", j)
				}
			}
			t.Logf("%d jobs; first: %+v", len(jobs), jobs[0])
		})
	}
}
