package service

import (
	"errors"
	"time"

	"learning-go/internal/events"
	"learning-go/internal/model"
	"learning-go/internal/repository"
)

var (
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotFound      = errors.New("user not found")
	ErrAlreadyExists = errors.New("user already exists")
)

type UserService struct {
	repo     repository.UserRepository
	eventBus *events.EventBus
}

func New(repo repository.UserRepository, eb *events.EventBus) *UserService {
	return &UserService{
		repo:     repo,
		eventBus: eb,
	}
}

func (s *UserService) CreateUser(name, email string) (*model.User, error) {
	if name == "" {
		return nil, ErrInvalidInput
	}
	if email == "" {
		return nil, ErrInvalidInput
	}

	users := s.repo.List()
	for _, u := range users {
		if u.Email == email {
			return nil, ErrAlreadyExists
		}
	}

	user := &model.User{
		ID:        "user-" + time.Now().Format("20060102150405"),
		Name:      name,
		Email:     email,
		Status:    model.StatusActive,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(user); err != nil {
		return nil, err
	}

	if s.eventBus != nil {
		s.eventBus.Publish(events.UserCreatedEvent{
			UserID:     user.ID,
			Email:      user.Email,
			Name:       user.Name,
			OccurredAt: time.Now(),
		})
	}

	return user, nil
}

func (s *UserService) GetUser(id string) (*model.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *UserService) UpdateUser(id, name string) (*model.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, ErrNotFound
	}

	user.Name = name
	user.UpdatedAt = time.Now()

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}

	if s.eventBus != nil {
		s.eventBus.Publish(events.UserUpdatedEvent{
			UserID:     user.ID,
			Name:       user.Name,
			OccurredAt: time.Now(),
		})
	}

	return user, nil
}

func (s *UserService) DeleteUser(id string) error {
	if err := s.repo.Delete(id); err != nil {
		return ErrNotFound
	}

	if s.eventBus != nil {
		s.eventBus.Publish(events.UserDeletedEvent{
			UserID:     id,
			OccurredAt: time.Now(),
		})
	}

	return nil
}

func (s *UserService) ListUsers() []*model.User {
	return s.repo.List()
}
