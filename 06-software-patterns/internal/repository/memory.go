package repository

import (
	"errors"
	"sync"

	"learning-go/internal/model"
)

type userRepository struct {
	mu   sync.RWMutex
	data map[string]*model.User
}

func NewInMemory() UserRepository {
	return &userRepository{
		data: make(map[string]*model.User),
	}
}

func (r *userRepository) Create(user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[user.ID]; exists {
		return errors.New("user already exists")
	}

	r.data[user.ID] = user
	return nil
}

func (r *userRepository) GetByID(id string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.data[id]
	if !ok {
		return nil, model.ErrNotFound
	}

	return user, nil
}

func (r *userRepository) Update(user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[user.ID]; !exists {
		return model.ErrNotFound
	}

	r.data[user.ID] = user
	return nil
}

func (r *userRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.data[id]; !exists {
		return model.ErrNotFound
	}

	delete(r.data, id)
	return nil
}

func (r *userRepository) List() []*model.User {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]*model.User, 0, len(r.data))
	for _, u := range r.data {
		users = append(users, u)
	}

	return users
}
