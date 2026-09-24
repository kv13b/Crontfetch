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
			got := FilterJobs(all, tt.roles, tt.locations)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterJobs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
