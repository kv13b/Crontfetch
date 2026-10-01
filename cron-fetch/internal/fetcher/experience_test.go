package fetcher

import "testing"

func TestParseExperience(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantFound   bool
		wantMin     int // only checked if wantFound and wantMinSet
		wantMinSet  bool
		wantMax     int // only checked if wantFound and wantMaxSet
		wantMaxSet  bool
		wantDisplay string
	}{
		{"explicit range with hyphen", "Requirements: 3-5 years of experience", true, 3, true, 5, true, "3-5 years"},
		{"explicit range with en dash", "3–5 years of experience", true, 3, true, 5, true, "3-5 years"},
		{"explicit range with 'to'", "3 to 5 years of experience", true, 3, true, 5, true, "3-5 years"},
		{"range given backwards is normalized", "5-3 years of experience", true, 3, true, 5, true, "3-5 years"},
		{"plus with no upper bound", "5+ years of experience required", true, 5, true, 0, false, "5+ years"},
		{"up to, implies a zero lower bound", "up to 3 years of experience", true, 0, true, 3, true, "up to 3 years"},
		{"at least", "at least 2 years of experience", true, 2, true, 0, false, "2+ years"},
		{"minimum of", "minimum of 4 years of experience", true, 4, true, 0, false, "4+ years"},
		{"minimum without 'of'", "minimum 4 years of experience", true, 4, true, 0, false, "4+ years"},
		{"bare number near 'experience'", "3 years of experience required", true, 3, true, 0, false, "3+ years"},
		{"bare number with possessive", "3 years' experience required", true, 3, true, 0, false, "3+ years"},
		{"entry level with zero", "0-2 years of experience", true, 0, true, 2, true, "0-2 years"},
		{"no mention of years at all", "Bachelor's degree in Computer Science required", false, 0, false, 0, false, ""},
		{"a number that isn't experience-related", "Team of 5 engineers, posted in 2024", false, 0, false, 0, false, ""},
		{"range wins over a later bare mention", "3-5 years of experience. 10 years of industry presence.", true, 3, true, 5, true, "3-5 years"},
		{"HTML is stripped first", "<p><b>5+</b> years of experience</p>", true, 5, true, 0, false, "5+ years"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseExperience(stripHTML(tt.text))
			if got.found != tt.wantFound {
				t.Fatalf("found = %v, want %v", got.found, tt.wantFound)
			}
			if !tt.wantFound {
				return
			}
			if tt.wantMinSet != (got.min != nil) || (tt.wantMinSet && *got.min != tt.wantMin) {
				t.Errorf("min = %v, want set=%v value=%d", got.min, tt.wantMinSet, tt.wantMin)
			}
			if tt.wantMaxSet != (got.max != nil) || (tt.wantMaxSet && *got.max != tt.wantMax) {
				t.Errorf("max = %v, want set=%v value=%d", got.max, tt.wantMaxSet, tt.wantMax)
			}
			if got.display != tt.wantDisplay {
				t.Errorf("display = %q, want %q", got.display, tt.wantDisplay)
			}
		})
	}
}

func TestSupportsExperienceFilter(t *testing.T) {
	supported := []string{PlatformGreenhouse, PlatformLever, PlatformAshby, PlatformWorkable, PlatformRecruitee}
	for _, p := range supported {
		if !SupportsExperienceFilter(p) {
			t.Errorf("SupportsExperienceFilter(%q) = false, want true", p)
		}
	}

	unsupported := []string{PlatformTalentBrew, PlatformWorkday, PlatformSmartRecruiters, PlatformBeeSite, "", "unknown"}
	for _, p := range unsupported {
		if SupportsExperienceFilter(p) {
			t.Errorf("SupportsExperienceFilter(%q) = true, want false", p)
		}
	}
}

func TestJob_WithExperience(t *testing.T) {
	j := Job{ID: "1", Title: "Engineer"}.withExperience("<p>3-5 years of experience</p>")
	if j.Experience != "3-5 years" {
		t.Errorf("Experience = %q, want %q", j.Experience, "3-5 years")
	}

	// A job with no recognizable experience text keeps its fields nil/empty.
	j2 := Job{ID: "2", Title: "Engineer"}.withExperience("No experience requirement stated here.")
	if j2.Experience != "" || j2.expMin != nil || j2.expMax != nil {
		t.Errorf("unexpected experience set: %+v", j2)
	}
}
