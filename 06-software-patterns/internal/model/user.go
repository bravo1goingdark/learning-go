package model

import "errors"
import "time"

var ErrNotFound = errors.New("not found")

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)
