package models

import "time"

// User is the public auth identity. UI prefers `name`, falls back to `login`.
type User struct {
	ID          string    `json:"id"`
	Login       string    `json:"login"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}
