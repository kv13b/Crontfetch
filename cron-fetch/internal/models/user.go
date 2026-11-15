// Package models holds the data types that map onto CronFetch's database tables.
package models

import "time"

// User mirrors a row in the users table. PasswordHash is never serialized
// to JSON, so it can never accidentally leak in an API response.
type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Preferences is what a user is looking for, stored on their users row.
// The frontend uses it to fill the job filters.
type Preferences struct {
	Locations          []string `json:"locations"`
	Roles              []string `json:"roles"`
	Interests          []string `json:"interests"`
	MinExperienceYears *int     `json:"min_experience_years"`
	MaxExperienceYears *int     `json:"max_experience_years"`
}
