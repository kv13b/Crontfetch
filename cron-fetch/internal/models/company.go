package models

import "time"

// Company is a career page a user is tracking for job listings, along with
// the filters that decide which of its jobs are actually relevant.
type Company struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"-"`
	Name               string    `json:"name"`
	CareerURL          string    `json:"career_url"`
	MinExperienceYears *int      `json:"min_experience_years"`
	MaxExperienceYears *int      `json:"max_experience_years"`
	Roles              []string  `json:"roles"`
	Locations          []string  `json:"locations"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
