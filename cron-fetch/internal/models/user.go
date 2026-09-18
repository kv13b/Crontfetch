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
