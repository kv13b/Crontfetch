package handlers

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/kv13b/Crontfetch/internal/models"
)

func intPtr(n int) *int { return &n }

func baseCompany() models.Company {
	return models.Company{
		ID:                 "c1",
		Name:               "Valtech",
		CareerURL:          "https://www.valtech.com/en-in/career/jobs/",
		Platform:           "greenhouse",
		Board:              "valtech",
		MinExperienceYears: intPtr(3),
		MaxExperienceYears: intPtr(6),
		Roles:              []string{"developer"},
		Locations:          []string{"India"},
	}
}

func parsePatch(t *testing.T, body string) updateCompanyRequest {
	t.Helper()
	var p updateCompanyRequest
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("unmarshal %s: %v", body, err)
	}
	return p
}

func TestApplyCompanyPatch(t *testing.T) {
	tests := []struct {
		name  string
		patch string
		check func(t *testing.T, f companyFields)
	}{
		{"only min experience changes", `{"min_experience_years":5}`, func(t *testing.T, f companyFields) {
			if f.MinExperienceYears == nil || *f.MinExperienceYears != 5 {
				t.Errorf("min = %v, want 5", f.MinExperienceYears)
			}
			if *f.MaxExperienceYears != 6 || f.Name != "Valtech" || f.Platform != "greenhouse" ||
				f.Board != "valtech" || !reflect.DeepEqual(f.Roles, []string{"developer"}) {
				t.Errorf("other fields changed: %+v", f)
			}
		}},
		{"null clears an experience limit", `{"max_experience_years":null}`, func(t *testing.T, f companyFields) {
			if f.MaxExperienceYears != nil {
				t.Errorf("max = %v, want nil", *f.MaxExperienceYears)
			}
			if *f.MinExperienceYears != 3 {
				t.Errorf("min changed to %d", *f.MinExperienceYears)
			}
		}},
		{"empty list clears roles", `{"roles":[]}`, func(t *testing.T, f companyFields) {
			if f.Roles == nil || len(f.Roles) != 0 {
				t.Errorf("roles = %#v, want empty non-nil", f.Roles)
			}
		}},
		{"lists are replaced, not merged", `{"locations":["Bangalore","Remote"]}`, func(t *testing.T, f companyFields) {
			if !reflect.DeepEqual(f.Locations, []string{"Bangalore", "Remote"}) {
				t.Errorf("locations = %v", f.Locations)
			}
		}},
		{"new career_url resets platform and board for re-detection", `{"career_url":"https://jobs.lever.co/spotify"}`, func(t *testing.T, f companyFields) {
			if f.CareerURL != "https://jobs.lever.co/spotify" || f.Platform != "" || f.Board != "" {
				t.Errorf("got url=%q platform=%q board=%q", f.CareerURL, f.Platform, f.Board)
			}
		}},
		{"same career_url keeps platform and board", `{"career_url":"https://www.valtech.com/en-in/career/jobs/"}`, func(t *testing.T, f companyFields) {
			if f.Platform != "greenhouse" || f.Board != "valtech" {
				t.Errorf("platform=%q board=%q, want kept", f.Platform, f.Board)
			}
		}},
		{"new career_url with explicit platform and board keeps them", `{"career_url":"https://x.com/careers","platform":"lever","board":"x"}`, func(t *testing.T, f companyFields) {
			if f.Platform != "lever" || f.Board != "x" {
				t.Errorf("platform=%q board=%q", f.Platform, f.Board)
			}
		}},
		{"changing platform drops the old board", `{"platform":"lever"}`, func(t *testing.T, f companyFields) {
			if f.Platform != "lever" || f.Board != "" {
				t.Errorf("platform=%q board=%q, want lever with no board", f.Platform, f.Board)
			}
		}},
		{"resending the same platform keeps the board", `{"platform":"Greenhouse"}`, func(t *testing.T, f companyFields) {
			if f.Board != "valtech" {
				t.Errorf("board = %q, want valtech kept", f.Board)
			}
		}},
		{"empty platform means re-detect", `{"platform":""}`, func(t *testing.T, f companyFields) {
			if f.Platform != "" || f.Board != "" {
				t.Errorf("platform=%q board=%q, want both cleared", f.Platform, f.Board)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, applyCompanyPatch(baseCompany(), parsePatch(t, tt.patch)))
		})
	}
}

func TestUpdateCompanyRequest_IsEmpty(t *testing.T) {
	if !parsePatch(t, `{}`).isEmpty() {
		t.Error("{} should be empty")
	}
	if !parsePatch(t, `{"roles":null}`).isEmpty() {
		t.Error(`{"roles":null} should count as absent`)
	}
	for _, body := range []string{
		`{"name":"x"}`, `{"min_experience_years":null}`, `{"roles":[]}`, `{"platform":""}`, `{"board":""}`,
	} {
		if parsePatch(t, body).isEmpty() {
			t.Errorf("%s should not be empty", body)
		}
	}
}

func TestOptionalInt_RejectsNonNumbers(t *testing.T) {
	var p updateCompanyRequest
	if err := json.Unmarshal([]byte(`{"min_experience_years":"five"}`), &p); err == nil {
		t.Error("expected an error for a string value")
	}
}

// These cases all fail before platform detection, so no network is used.
func TestValidateCompany_Rejections(t *testing.T) {
	valid := func() companyFields {
		return companyFields{Name: "Acme", CareerURL: "https://acme.com/careers", Platform: "talentbrew"}
	}
	tests := []struct {
		name   string
		mutate func(f *companyFields)
		want   string
	}{
		{"blank name", func(f *companyFields) { f.Name = "  " }, "name and career_url are required"},
		{"non-http url", func(f *companyFields) { f.CareerURL = "acme.com/careers" }, "career_url must start with"},
		{"unknown platform", func(f *companyFields) { f.Platform = "taleo" }, "platform must be one of"},
		{"board without platform", func(f *companyFields) { f.Platform = ""; f.Board = "x" }, "platform is required when board is set"},
		{"lever without a board", func(f *companyFields) { f.Platform = "lever" }, "board is required for lever"},
		{"workday on a non-workday url", func(f *companyFields) { f.Platform = "workday" }, "myworkdayjobs.com"},
		{"beesite over http", func(f *companyFields) { f.Platform = "beesite"; f.Board = "http://internal.local" }, "BeeSite API address"},
		{"negative min", func(f *companyFields) { f.MinExperienceYears = intPtr(-1) }, "min_experience_years cannot be negative"},
		{"negative max", func(f *companyFields) { f.MaxExperienceYears = intPtr(-2) }, "max_experience_years cannot be negative"},
		{"min above max", func(f *companyFields) { f.MinExperienceYears = intPtr(8); f.MaxExperienceYears = intPtr(3) }, "cannot be greater"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := valid()
			tt.mutate(&f)
			if msg := validateCompany(context.Background(), &f); !strings.Contains(msg, tt.want) {
				t.Errorf("message = %q, want it to contain %q", msg, tt.want)
			}
		})
	}
}

func TestValidateCompany_TidiesInput(t *testing.T) {
	f := companyFields{
		Name:      "  Acme ",
		CareerURL: " https://acme.com/careers ",
		Platform:  " TalentBrew ",
		Roles:     []string{" developer ", "", "  "},
	}
	if msg := validateCompany(context.Background(), &f); msg != "" {
		t.Fatalf("unexpected rejection: %s", msg)
	}
	if f.Name != "Acme" || f.CareerURL != "https://acme.com/careers" || f.Platform != "talentbrew" {
		t.Errorf("not tidied: %+v", f)
	}
	if !reflect.DeepEqual(f.Roles, []string{"developer"}) || f.Locations == nil {
		t.Errorf("roles=%#v locations=%#v", f.Roles, f.Locations)
	}
}
