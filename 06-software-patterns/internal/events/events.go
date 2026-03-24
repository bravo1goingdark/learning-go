package events

import "time"

type Event interface {
	Type() string
	Timestamp() time.Time
}

type UserCreatedEvent struct {
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e UserCreatedEvent) Type() string         { return "user.created" }
func (e UserCreatedEvent) Timestamp() time.Time { return e.OccurredAt }

type UserUpdatedEvent struct {
	UserID     string    `json:"user_id"`
	Name       string    `json:"name"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e UserUpdatedEvent) Type() string         { return "user.updated" }
func (e UserUpdatedEvent) Timestamp() time.Time { return e.OccurredAt }

type UserDeletedEvent struct {
	UserID     string    `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

func (e UserDeletedEvent) Type() string         { return "user.deleted" }
func (e UserDeletedEvent) Timestamp() time.Time { return e.OccurredAt }
