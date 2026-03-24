package repository

import "learning-go/internal/model"

type UserRepository interface {
	Create(user *model.User) error
	GetByID(id string) (*model.User, error)
	Update(user *model.User) error
	Delete(id string) error
	List() []*model.User
}
