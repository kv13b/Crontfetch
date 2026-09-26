package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScanPage(t *testing.T) {
	tests := []struct {
		name string
		html string
		want Detection
	}{
		{"greenhouse embed script", `<script src="https://boards.greenhouse.io/embed/job_board/js?for=Acme"></script>`, Detection{PlatformGreenhouse, "acme"}},
		{"greenhouse embed skips the word embed", `<a href="https://boards.greenhouse.io/embed/job_board">x</a> <a href="https://boards.greenhouse.io/acme/jobs/1">y</a>`, Detection{PlatformGreenhouse, "acme"}},
		{"greenhouse job link", `<a href="https://job-boards.greenhouse.io/acme/jobs/42">`, Detection{PlatformGreenhouse, "acme"}},
		{"greenhouse api reference", `fetch("https://boards-api.greenhouse.io/v1/boards/acme/jobs")`, Detection{PlatformGreenhouse, "acme"}},
		{"greenhouse tracking param only", `<a href="/jobs?gh_jid=123">`, Detection{Platform: PlatformGreenhouse}},
		{"workday link, locale skipped", `<a href="https://acme.wd5.myworkdayjobs.com/en-US/AcmeCareers/job/x">`, Detection{PlatformWorkday, "https://acme.wd5.myworkdayjobs.com/AcmeCareers"}},
		{"workday asset path ignored", `<script src="https://acme.wd5.myworkdayjobs.com/assets/app.js">`, Detection{}},
		{"talentbrew", `<link href="//tbcdn.talentbrew.com/company/1/css/a.css">`, Detection{Platform: PlatformTalentBrew}},
		{"talentbrew outranks a workday link", `<link href="//tbcdn.talentbrew.com/a.css"><a href="https://acme.wd5.myworkdayjobs.com/en-US/Other/x">`, Detection{Platform: PlatformTalentBrew}},
		{"lever job link", `<a href="https://jobs.lever.co/spotify/2193db3f">`, Detection{PlatformLever, "spotify"}},
		{"lever api call", `fetch("https://api.lever.co/v0/postings/spotify?mode=json")`, Detection{PlatformLever, "spotify"}},
		{"ashby board", `<a href="https://jobs.ashbyhq.com/linear/d3bc1ced">`, Detection{PlatformAshby, "linear"}},
		{"ashby api path isn't a board", `<script src="https://jobs.ashbyhq.com/api/x.js"></script>`, Detection{}},
		{"smartrecruiters careers link", `<a href="https://careers.smartrecruiters.com/BoschGroup">`, Detection{PlatformSmartRecruiters, "BoschGroup"}},
		{"smartrecruiters api", `https://api.smartrecruiters.com/v1/companies/BoschGroup/postings`, Detection{PlatformSmartRecruiters, "BoschGroup"}},
		{"workable board", `<a href="https://apply.workable.com/huggingface/j/F4C096B22E/">`, Detection{PlatformWorkable, "huggingface"}},
		{"workable job path isn't a board", `<a href="https://apply.workable.com/j/F4C096B22E">`, Detection{}},
		{"recruitee subdomain", `<a href="https://bunq.recruitee.com/o/legal-counsel">`, Detection{PlatformRecruitee, "bunq"}},
		{"recruitee www isn't a board", `<a href="https://www.recruitee.com/pricing">`, Detection{}},
		{"known but unsupported vendor", `<a href="https://acme.taleo.net/careersection/2/jobsearch.ftl">`, Detection{Platform: "taleo"}},
		{"nothing recognisable", `<html><body>Careers at Acme</body></html>`, Detection{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scanPage(tt.html); got != tt.want {
				t.Errorf("scanPage() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCompanyLabel(t *testing.T) {
	tests := map[string]string{
		"www.valtech.com":           "valtech",
		"jobs.paloaltonetworks.com": "paloaltonetworks",
		"careers.acme.co.uk":        "acme",
		"acme.com.au":               "acme",
		"acme.io":                   "acme",
		"localhost":                 "",
		"127.0.0.1":                 "",
	}
	for host, want := range tests {
		if got := companyLabel(host); got != want {
			t.Errorf("companyLabel(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestNamesMatch(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"Valtech", "valtech", true},
		{"Palo Alto Networks", "paloaltonetworks", true},
		{"Acme Inc.", "acme", true},
		{"Delta Air Lines", "delta", true},
		{"Target Corp", "acme", false},
		{"", "acme", false},
	}
	for _, tt := range tests {
		if got := namesMatch(tt.a, tt.b); got != tt.want {
			t.Errorf("namesMatch(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// fakeGreenhouse serves board-name lookups: "valtech" is Valtech's board;
// "delta" belongs to some unrelated company; anything else is unknown.
func fakeGreenhouse(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valtech":
			w.Write([]byte(`{"name":"Valtech","content":""}`))
		case "/delta":
			w.Write([]byte(`{"name":"Delta Robotics Lab","content":""}`))
		case "/unrelated":
			w.Write([]byte(`{"name":"Completely Different Co","content":""}`))
		default:
			http.NotFound(w, r)
		}
	}))
	old := greenhouseAPIBase
	greenhouseAPIBase = srv.URL
	t.Cleanup(func() { greenhouseAPIBase = old; srv.Close() })
}

func TestGuessGreenhouseBoard(t *testing.T) {
	fakeGreenhouse(t)
	ctx := context.Background()

	if got := guessGreenhouseBoard(ctx, "https://www.valtech.com/en-in/career/jobs/"); got != "valtech" {
		t.Errorf("valtech guess = %q, want valtech", got)
	}
	if got := guessGreenhouseBoard(ctx, "https://www.nosuchcompany.com/careers"); got != "" {
		t.Errorf("unknown board guess = %q, want empty", got)
	}
	// The board exists but its registered name doesn't match the domain.
	if got := guessGreenhouseBoard(ctx, "https://www.unrelated.com/careers"); got != "" {
		t.Errorf("mismatched-name guess = %q, want empty", got)
	}
}

func servePage(t *testing.T, html string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(html))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDetect_FromPage(t *testing.T) {
	srv := servePage(t, `<iframe src="https://boards.greenhouse.io/embed/job_board?for=acme"></iframe>`)

	got := Detect(context.Background(), srv.URL)
	if got != (Detection{PlatformGreenhouse, "acme"}) {
		t.Errorf("Detect() = %+v", got)
	}
	if !got.Usable() {
		t.Error("expected a usable detection")
	}
}

func TestDetect_UnsupportedVendorIsNotOverriddenByGuess(t *testing.T) {
	fakeGreenhouse(t)
	srv := servePage(t, `<a href="https://acme.taleo.net/careersection/2/jobsearch.ftl">`)

	// Host is 127.0.0.1 so no board guess is possible here; the point is
	// that a recognised vendor is reported, and usable is false.
	got := Detect(context.Background(), srv.URL)
	if got.Platform != "taleo" || got.Usable() {
		t.Errorf("Detect() = %+v, want unusable taleo", got)
	}
}

func TestFetchJobs_UnsupportedVendor(t *testing.T) {
	srv := servePage(t, `<a href="https://acme.taleo.net/careersection/2/jobsearch.ftl">`)

	_, err := FetchJobs(context.Background(), srv.URL, "", "", 1)

	var unsupported *UnsupportedPlatformError
	if !errors.As(err, &unsupported) || unsupported.Platform != "taleo" {
		t.Fatalf("error = %v, want UnsupportedPlatformError for taleo", err)
	}
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Error("UnsupportedPlatformError should also match ErrUnsupportedPlatform")
	}
}

// Live checks against real career sites.
func TestDetect_Live(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want Detection
	}{
		{"wrapper on greenhouse, board named after domain", "https://www.valtech.com/en-in/career/jobs/", Detection{PlatformGreenhouse, "valtech"}},
		{"cloudflare wrapper", "https://www.cloudflare.com/careers/jobs/", Detection{PlatformGreenhouse, "cloudflare"}},
		{"talentbrew page that also links to workday", "https://jobs.paloaltonetworks.com/en", Detection{Platform: PlatformTalentBrew}},
		{"direct workday url", "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite", Detection{Platform: PlatformWorkday}},
		{"unrelated site", "https://example.com/careers", Detection{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(context.Background(), tt.url); got != tt.want {
				t.Errorf("Detect(%s) = %+v, want %+v", tt.url, got, tt.want)
			}
		})
	}
}
