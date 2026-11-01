package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// fakeBoards serves every platform's API under its own path prefix and
// answers 404 unless a handler for that path is given.
func fakeBoards(t *testing.T, routes map[string]string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := routes[r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	old := []string{greenhouseAPIBase, workableAPIBase, smartRecruitersAPIBase, leverAPIBase, ashbyAPIBase}
	oldRecruitee, oldSites := recruiteeAPIURL, guessedSites
	greenhouseAPIBase = srv.URL + "/gh"
	workableAPIBase = srv.URL + "/wk"
	smartRecruitersAPIBase = srv.URL + "/sr"
	leverAPIBase = srv.URL + "/lv"
	ashbyAPIBase = srv.URL + "/ab"
	recruiteeAPIURL = func(board string) string { return srv.URL + "/rc/" + board }
	// No guessed websites unless a test sets them.
	guessedSites = func(string) []string { return nil }
	t.Cleanup(func() {
		greenhouseAPIBase, workableAPIBase, smartRecruitersAPIBase, leverAPIBase, ashbyAPIBase = old[0], old[1], old[2], old[3], old[4]
		recruiteeAPIURL, guessedSites = oldRecruitee, oldSites
	})
}

func TestNameSlugs(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"Altisource Business Solutions Pvt Ltd", []string{"altisource", "altisourcebusinesssolutionspvtltd", "altisource-business-solutions-pvt-ltd"}},
		{"Walmart Labs", []string{"walmart", "walmartlabs", "walmart-labs"}},
		{"AT&T", []string{"att", "at-t"}},
		{"IBM", []string{"ibm"}},
		{"", nil},
	}
	for _, tt := range tests {
		var got []string
		for _, s := range nameSlugs(tt.name) {
			got = append(got, s.value)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("nameSlugs(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFindCareerPage_VerifiedBoardByName(t *testing.T) {
	fakeBoards(t, map[string]string{
		"/gh/bazaarvoice":      `{"name":"Bazaarvoice"}`,
		"/gh/bazaarvoice/jobs": `{"jobs":[{"id":1}]}`,
	})

	got := FindCareerPage(context.Background(), "Bazaarvoice Technology India", "")
	if !got.Found || got.Platform != PlatformGreenhouse || got.URL != "https://job-boards.greenhouse.io/bazaarvoice" {
		t.Fatalf("FindCareerPage() = %+v, want the Greenhouse board", got)
	}
	// The suggested URL must be recognised on its own when the company is saved.
	if p, b := DetectPlatform(got.URL); p != PlatformGreenhouse || b != "bazaarvoice" {
		t.Errorf("DetectPlatform(%q) = %q, %q", got.URL, p, b)
	}
}

func TestFindCareerPage_RejectsBoardWithAnotherCompanysName(t *testing.T) {
	fakeBoards(t, map[string]string{
		"/gh/apex":      `{"name":"Totally Different Ltd"}`,
		"/gh/apex/jobs": `{"jobs":[{"id":1}]}`,
	})

	if got := FindCareerPage(context.Background(), "Apex", ""); got.Found {
		t.Errorf("FindCareerPage() = %+v, want nothing (board belongs to another company)", got)
	}
}

func TestFindCareerPage_LeverRanksBelowVerifiedBoard(t *testing.T) {
	fakeBoards(t, map[string]string{
		"/lv/rippling": `[{"id":"1"}]`,
		"/wk/rippling": `{"name":"Rippling","jobs":[{"id":"1"}]}`,
	})
	got := FindCareerPage(context.Background(), "Rippling", "")
	if got.Platform != PlatformWorkable {
		t.Errorf("FindCareerPage() = %+v, want the name-verified Workable board over unverified Lever", got)
	}
}

func TestFindCareerPage_OwnWebsiteWins(t *testing.T) {
	fakeBoards(t, map[string]string{"/gh/acme": `{"name":"Acme"}`, "/gh/acme/jobs": `{"jobs":[{"id":1}]}`})
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<a href="/careers">Careers</a>`))
		case "/careers":
			w.Write([]byte(`<a href="https://jobs.lever.co/acme-inc/123">Apply</a>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()

	got := FindCareerPage(context.Background(), "Acme", site.URL)
	if got.Platform != PlatformLever || got.Board != "acme-inc" || got.URL != "https://jobs.lever.co/acme-inc" {
		t.Fatalf("FindCareerPage() = %+v, want the Lever board linked from the company's own site", got)
	}
	if !strings.Contains(got.How, "/careers") {
		t.Errorf("How = %q, want it to say where the link was found", got.How)
	}
}

func TestFindCareerPage_IgnoresBoardsWithNoJobs(t *testing.T) {
	fakeBoards(t, map[string]string{
		"/wk/deloitte":      `{"name":"Deloitte","jobs":[]}`, // dormant account
		"/gh/deloitte":      `{"name":"Deloitte"}`,
		"/gh/deloitte/jobs": `{"jobs":[]}`,
	})
	if got := FindCareerPage(context.Background(), "Deloitte", ""); got.Found {
		t.Errorf("FindCareerPage() = %+v, want nothing (boards with no jobs)", got)
	}
}

func TestFindCareerPage_CareersPageWithoutKnownPlatform(t *testing.T) {
	fakeBoards(t, nil)
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`<h1>Home</h1>`))
		case "/jobs":
			w.Write([]byte(`<h1>Work with us</h1>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()

	got := FindCareerPage(context.Background(), "Nobody Known", site.URL)
	if got.Found || got.URL != site.URL+"/jobs" {
		t.Errorf("FindCareerPage() = %+v, want the /jobs page with Found false", got)
	}
}

func TestFindCareerPage_NothingFound(t *testing.T) {
	fakeBoards(t, nil)
	got := FindCareerPage(context.Background(), "Nobody Known", "")
	if got.Found || got.URL != "" {
		t.Errorf("FindCareerPage() = %+v, want an empty suggestion", got)
	}
}

func TestNormalizeWebsite(t *testing.T) {
	tests := map[string]string{
		"saifpartners.com":         "https://saifpartners.com",
		" https://nuvira.digital ": "https://nuvira.digital",
		"www.platify.co":           "https://www.platify.co",
		"ftp://example.com":        "",
		"":                         "",
	}
	for in, want := range tests {
		if got := normalizeWebsite(in); got != want {
			t.Errorf("normalizeWebsite(%q) = %q, want %q", in, got, want)
		}
	}
}
