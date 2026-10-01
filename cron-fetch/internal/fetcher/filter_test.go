package fetcher

import (
	"reflect"
	"testing"
)

func TestFilterJobs(t *testing.T) {
	blr := Job{ID: "1", Title: "Staff Software Engineer", Location: "Bangalore, Karnātaka, India"}
	sg := Job{ID: "2", Title: "Software Engineer", Location: "Singapore, Central Singapore, Singapore"}
	mgr := Job{ID: "3", Title: "Product Manager", Location: "Bangalore, Karnātaka, India"}
	remote := Job{ID: "4", Title: "Remote Backend Developer", Location: "New York, United States"}
	all := []Job{blr, sg, mgr, remote}

	tests := []struct {
		name      string
		roles     []string
		locations []string
		want      []Job
	}{
		{"no filters keeps everything", nil, nil, all},
		{"role is case-insensitive substring of title", []string{"SOFTWARE ENGINEER"}, nil, []Job{blr, sg}},
		{"any role may match", []string{"software engineer", "developer"}, nil, []Job{blr, sg, remote}},
		{"location parts match the site's longer format", nil, []string{"Bangalore, India"}, []Job{blr, mgr}},
		{"any location may match", nil, []string{"Bangalore, India", "Singapore"}, []Job{blr, sg, mgr}},
		{"remote matches the title", nil, []string{"Remote"}, []Job{remote}},
		{"roles and locations must both match", []string{"software engineer"}, []string{"Bangalore, India"}, []Job{blr}},
		{"nothing matches", []string{"designer"}, nil, []Job{}},
		{"filter with only separators matches nothing", nil, []string{" , "}, []Job{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterJobs(all, tt.roles, tt.locations, nil, nil)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterJobs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFilterJobs_Experience(t *testing.T) {
	junior := Job{ID: "1", Title: "Engineer"}.withExperience("0-2 years of experience required")
	mid := Job{ID: "2", Title: "Engineer"}.withExperience("3-5 years of experience required")
	senior := Job{ID: "3", Title: "Engineer"}.withExperience("8+ years of experience required")
	unstated := Job{ID: "4", Title: "Engineer"} // no description available for this platform
	all := []Job{junior, mid, senior, unstated}

	tests := []struct {
		name     string
		min, max *int
		want     []Job
	}{
		{"no constraint keeps everything", nil, nil, all},
		{"mid-range filter: junior and mid overlap [2,6], senior (8+) is too senior", intPtr(2), intPtr(6), []Job{junior, mid, unstated}},
		{"min only: senior and unstated qualify, junior/mid cap out too low", intPtr(6), nil, []Job{senior, unstated}},
		{"max only: junior and unstated qualify, mid/senior start too high", nil, intPtr(2), []Job{junior, unstated}},
		{"high range: open-ended senior (8+) still overlaps, bounded ones don't", intPtr(20), intPtr(25), []Job{senior, unstated}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterJobs(all, nil, nil, tt.min, tt.max)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterJobs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
