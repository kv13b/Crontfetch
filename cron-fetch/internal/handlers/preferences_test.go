package handlers

import (
	"reflect"
	"testing"

	"github.com/kv13b/Crontfetch/internal/models"
)

func TestValidatePreferences(t *testing.T) {
	tests := []struct {
		name    string
		in      models.Preferences
		wantErr string
	}{
		{name: "empty is valid", in: models.Preferences{}},
		{name: "min only", in: models.Preferences{MinExperienceYears: intPtr(2)}},
		{name: "min equals max", in: models.Preferences{MinExperienceYears: intPtr(3), MaxExperienceYears: intPtr(3)}},
		{name: "negative min", in: models.Preferences{MinExperienceYears: intPtr(-1)}, wantErr: "min_experience_years cannot be negative"},
		{name: "negative max", in: models.Preferences{MaxExperienceYears: intPtr(-1)}, wantErr: "max_experience_years cannot be negative"},
		{name: "min above max", in: models.Preferences{MinExperienceYears: intPtr(5), MaxExperienceYears: intPtr(2)}, wantErr: "min_experience_years cannot be greater than max_experience_years"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validatePreferences(&tt.in); got != tt.wantErr {
				t.Errorf("validatePreferences() = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

func TestValidatePreferencesCleansLists(t *testing.T) {
	p := models.Preferences{Locations: []string{" Bangalore ", "", "  "}}
	if msg := validatePreferences(&p); msg != "" {
		t.Fatalf("unexpected error %q", msg)
	}
	if !reflect.DeepEqual(p.Locations, []string{"Bangalore"}) {
		t.Errorf("Locations = %q", p.Locations)
	}
	// Omitted lists must become empty, not nil, so the NOT NULL columns accept them.
	if p.Roles == nil || p.Interests == nil {
		t.Errorf("Roles/Interests should be empty slices, got %#v %#v", p.Roles, p.Interests)
	}
}
