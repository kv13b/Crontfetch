package models

import "time"

// Company is a career page a user is tracking for job listings, along with
// the filters that decide which of its jobs are actually relevant.
type Company struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"-"`
	Name               string    `json:"name"`
	CareerURL          string    `json:"career_url"`
	Platform           string    `json:"platform"`
	Board              string    `json:"board"`
	MinExperienceYears *int      `json:"min_experience_years"`
	MaxExperienceYears *int      `json:"max_experience_years"`
	Roles              []string  `json:"roles"`
	Locations          []string  `json:"locations"`
	OfficeAddress      string    `json:"office_address"`
	Lat                *float64  `json:"lat"`
	Lng                *float64  `json:"lng"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
