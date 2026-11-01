package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// CareerPage is a suggested career page for a company, for the user to
// check before saving.
type CareerPage struct {
	URL      string `json:"career_url"`
	Platform string `json:"platform"`
	Board    string `json:"board"`
	// Found means a job board we can read was found, not just a page.
	Found bool `json:"found"`
	// How says in plain words how it was found, for the user to judge it.
	How string `json:"how"`
}

// careerPaths are tried on a company's own site when its home page doesn't
// reveal a job board.
var careerPaths = []string{"/careers", "/jobs", "/career", "/join-us"}

// nameNoise are words dropped from a company name when guessing its board
// name: "Altisource Business Solutions Pvt Ltd" -> "altisource".
var nameNoise = map[string]bool{
	"pvt": true, "private": true, "ltd": true, "limited": true, "inc": true, "llc": true, "llp": true,
	"co": true, "corp": true, "corporation": true, "company": true, "the": true, "and": true,
	"india": true, "technologies": true, "technology": true, "solutions": true, "services": true,
	"software": true, "systems": true, "labs": true, "international": true, "global": true,
	"group": true, "office": true, "campus": true, "centre": true, "center": true, "business": true,
	"digital": true, "foundation": true, "associates": true, "consulting": true,
}

var wordRe = regexp.MustCompile(`[a-z0-9]+`)

// candidate ranks, best first. Lower wins.
const (
	rankOwnSite      = iota // a job board linked from the company's own website
	rankVerifiedName        // a board named after the company whose registered name matches
	rankGuessedSite         // a job board linked from www.<name>.com
	rankNameOnly            // a Lever/Ashby board with the company's exact name, unverified
	rankCareersPage         // a careers page on the company's own site, platform unknown
	rankNone
)

type careerCandidate struct {
	rank int
	page CareerPage
}

// guessedSites returns websites to try for a company with none on record.
// A variable so tests can point it at a local server.
var guessedSites = func(slug string) []string {
	return []string{"https://www." + slug + ".com"}
}

// FindCareerPage suggests a career page for a company from its name and, if
// known, its website. It looks in two ways at once:
//   - on the company's website (or www.<name>.com): job-board links on the
//     home page, then on common paths like /careers;
//   - on the job boards themselves, by trying board names made from the
//     company name. Boards that report a company name must match it.
//
// The best evidence wins (see the rank constants). With nothing found it
// returns the website, if any, with Found false.
func FindCareerPage(ctx context.Context, name, website string) CareerPage {
	website = normalizeWebsite(website)
	slugs := nameSlugs(name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu   sync.Mutex
		best = careerCandidate{rank: rankNone}
		wg   sync.WaitGroup
	)
	offer := func(c careerCandidate) {
		mu.Lock()
		defer mu.Unlock()
		if c.rank < best.rank {
			best = c
		}
		// Stop early once nothing still running could do better: only a
		// link on the company's own website beats a name-verified board.
		if best.rank == rankOwnSite || (best.rank == rankVerifiedName && website == "") {
			cancel()
		}
	}

	wg.Add(2)
	go func() {
		defer wg.Done()
		if website != "" {
			searchSite(ctx, website, rankOwnSite, true, offer)
			return
		}
		if len(slugs) > 0 {
			for _, site := range guessedSites(slugs[0].value) {
				searchSite(ctx, site, rankGuessedSite, false, offer)
			}
		}
	}()
	go func() {
		defer wg.Done()
		searchBoards(ctx, name, slugs, offer)
	}()
	wg.Wait()

	if best.rank == rankNone {
		return CareerPage{URL: website, How: "no job board found"}
	}
	return best.page
}

// searchSite looks for a job board on a website: the home page first, then
// common career paths. When keepPage is set, a careers page that exists but
// shows no known platform is still offered, as a weak candidate.
func searchSite(ctx context.Context, site string, rank int, keepPage bool, offer func(careerCandidate)) {
	if d := Detect(ctx, site); d.Usable() {
		offer(careerCandidate{rank, pageFromDetection(d, site, "linked from "+hostOf(site))})
		return
	}
	base := strings.TrimRight(site, "/")
	for _, p := range careerPaths {
		html, finalURL, ok := fetchPageURL(ctx, base+p)
		if !ok {
			continue
		}
		if d := scanPage(html); d.Usable() {
			offer(careerCandidate{rank, pageFromDetection(d, finalURL, "linked from "+hostOf(finalURL)+pathOf(finalURL))})
			return
		}
		// A soft 404 often redirects to the home page; only keep pages
		// that still look like a careers page.
		if keepPage && looksLikeCareersURL(finalURL) {
			offer(careerCandidate{rankCareersPage, CareerPage{URL: finalURL, How: "careers page on the company's website; job board not recognised"}})
			return
		}
	}
}

type nameSlug struct {
	value string
	exact bool // made from the whole name (or the name without noise words)
}

// nameSlugs turns a company name into likely board names, most likely
// first: "Walmart Labs" -> walmartlabs, walmart-labs, walmart.
func nameSlugs(name string) []nameSlug {
	words := wordRe.FindAllString(strings.ToLower(name), -1)
	if len(words) == 0 {
		return nil
	}
	core := make([]string, 0, len(words))
	for _, w := range words {
		if !nameNoise[w] {
			core = append(core, w)
		}
	}
	if len(core) == 0 {
		core = words
	}

	var out []nameSlug
	seen := map[string]bool{}
	add := func(v string, exact bool) {
		if len(v) >= 2 && !seen[v] {
			seen[v] = true
			out = append(out, nameSlug{v, exact})
		}
	}
	add(strings.Join(core, ""), true)
	add(strings.Join(core, "-"), true)
	add(strings.Join(words, ""), true)
	add(strings.Join(words, "-"), true)
	if len(core[0]) >= 4 {
		add(core[0], false)
	}
	return out
}

// searchBoards tries each board name on each platform that has a public
// API. Greenhouse, Workable, SmartRecruiters and Recruitee report the
// company's name, which must match; Lever and Ashby don't, so they're only
// tried with names made from the whole company name, and rank lower.
func searchBoards(ctx context.Context, name string, slugs []nameSlug, offer func(careerCandidate)) {
	type probe struct {
		platform string
		slug     nameSlug
	}
	var probes []probe
	for _, s := range slugs {
		for _, p := range []string{PlatformGreenhouse, PlatformWorkable, PlatformSmartRecruiters, PlatformRecruitee} {
			probes = append(probes, probe{p, s})
		}
		if s.exact {
			probes = append(probes, probe{PlatformLever, s}, probe{PlatformAshby, s})
		}
	}

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, pr := range probes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if boardName, ok := boardExists(ctx, pr.platform, pr.slug.value); ok {
				rank, how := rankVerifiedName, fmt.Sprintf("%s board %q (registered as %q)", platformTitle(pr.platform), pr.slug.value, boardName)
				if boardName == "" {
					rank, how = rankNameOnly, fmt.Sprintf("%s board %q, named after the company", platformTitle(pr.platform), pr.slug.value)
				} else if !namesMatch(boardName, name) {
					return
				}
				d := Detection{Platform: pr.platform, Board: pr.slug.value}
				offer(careerCandidate{rank, pageFromDetection(d, "", how)})
			}
		}()
	}
	wg.Wait()
}

// boardExists checks that a board exists on one platform and has at least
// one open job (an empty board is no use to track, and empty accounts are
// how squatted brand names show up). It returns the company name the
// platform reports, or "" for platforms that don't report one.
func boardExists(ctx context.Context, platform, slug string) (string, bool) {
	esc := url.PathEscape(slug)
	switch platform {
	case PlatformGreenhouse:
		var b struct {
			Name string `json:"name"`
		}
		if !getJSON(ctx, greenhouseAPIBase+"/"+esc, &b) || b.Name == "" {
			return "", false
		}
		var jobs struct {
			Jobs []json.RawMessage `json:"jobs"`
		}
		ok := getJSON(ctx, greenhouseAPIBase+"/"+esc+"/jobs", &jobs)
		return b.Name, ok && len(jobs.Jobs) > 0
	case PlatformWorkable:
		// Workable keeps dormant accounts under many big brand names
		// ("IBM", "Deloitte") with no jobs, so a board must have some.
		var b struct {
			Name string            `json:"name"`
			Jobs []json.RawMessage `json:"jobs"`
		}
		ok := getJSON(ctx, workableAPIBase+"/"+esc, &b)
		return b.Name, ok && b.Name != "" && len(b.Jobs) > 0
	case PlatformSmartRecruiters:
		// Answers 200 with no postings for a company it doesn't know.
		var b struct {
			TotalFound int `json:"totalFound"`
			Content    []struct {
				Company struct {
					Name string `json:"name"`
				} `json:"company"`
			} `json:"content"`
		}
		ok := getJSON(ctx, smartRecruitersAPIBase+"/"+esc+"/postings?limit=1", &b)
		if !ok || b.TotalFound == 0 || len(b.Content) == 0 {
			return "", false
		}
		return b.Content[0].Company.Name, b.Content[0].Company.Name != ""
	case PlatformRecruitee:
		var b struct {
			Offers []struct {
				CompanyName string `json:"company_name"`
			} `json:"offers"`
		}
		ok := getJSON(ctx, recruiteeAPIURL(esc), &b)
		if !ok || len(b.Offers) == 0 {
			return "", false
		}
		return b.Offers[0].CompanyName, b.Offers[0].CompanyName != ""
	case PlatformLever:
		var postings []json.RawMessage
		ok := getJSON(ctx, leverAPIBase+"/"+esc+"?mode=json&limit=1", &postings)
		return "", ok && len(postings) > 0
	case PlatformAshby:
		var b struct {
			Jobs []json.RawMessage `json:"jobs"`
		}
		ok := getJSON(ctx, ashbyAPIBase+"/"+esc, &b)
		return "", ok && len(b.Jobs) > 0
	}
	return "", false
}

func getJSON(ctx context.Context, apiURL string, v any) bool {
	resp, err := get(ctx, apiURL, false)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(v) == nil
}

// fetchPageURL is fetchPage plus the URL it ended up at after redirects.
func fetchPageURL(ctx context.Context, pageURL string) (html, finalURL string, ok bool) {
	resp, err := get(ctx, pageURL, false)
	if err != nil {
		return "", "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDetectBody))
	if err != nil {
		return "", "", false
	}
	return string(body), resp.Request.URL.String(), true
}

// pageFromDetection builds the suggestion. Board-based platforms get their
// public board page, which DetectPlatform recognises from the URL alone, so
// saving the company needs no second detection.
func pageFromDetection(d Detection, pageURL, how string) CareerPage {
	p := CareerPage{Platform: d.Platform, Board: d.Board, Found: true, How: how}
	switch d.Platform {
	case PlatformGreenhouse:
		p.URL = "https://job-boards.greenhouse.io/" + d.Board
	case PlatformLever:
		p.URL = "https://jobs.lever.co/" + d.Board
	case PlatformAshby:
		p.URL = "https://jobs.ashbyhq.com/" + d.Board
	case PlatformSmartRecruiters:
		p.URL = "https://jobs.smartrecruiters.com/" + d.Board
	case PlatformWorkable:
		p.URL = "https://apply.workable.com/" + d.Board + "/"
	case PlatformRecruitee:
		p.URL = "https://" + d.Board + ".recruitee.com/"
	case PlatformWorkday:
		p.URL = d.Board // the career site's own URL
		if p.URL == "" {
			p.URL = pageURL
		}
	default: // TalentBrew, BeeSite: the page where it was found
		p.URL = pageURL
	}
	return p
}

func platformTitle(p string) string {
	switch p {
	case PlatformSmartRecruiters:
		return "SmartRecruiters"
	case PlatformTalentBrew:
		return "TalentBrew"
	case PlatformBeeSite:
		return "BeeSite"
	}
	if p == "" {
		return p
	}
	return strings.ToUpper(p[:1]) + p[1:]
}

// normalizeWebsite adds a scheme to bare domains ("saifpartners.com"), as
// OpenStreetMap often stores them, and drops anything that isn't a web URL.
func normalizeWebsite(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return s
}

func looksLikeCareersURL(u string) bool {
	p := strings.ToLower(pathOf(u))
	return strings.Contains(p, "career") || strings.Contains(p, "job") || strings.Contains(p, "join")
}

func hostOf(u string) string {
	if parsed, err := url.Parse(u); err == nil {
		return parsed.Host
	}
	return u
}

func pathOf(u string) string {
	if parsed, err := url.Parse(u); err == nil && parsed.Path != "/" {
		return parsed.Path
	}
	return ""
}
