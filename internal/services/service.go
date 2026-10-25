package services

import (
	"errors"
	"strings"

	"github.com/quan/task-management-api/internal/models"
	"github.com/quan/task-management-api/internal/repositories"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrInvalid = errors.New("invalid input")

type Service struct {
	Users    repositories.Repository[models.User]
	Projects repositories.Repository[models.Project]
	Tasks    repositories.Repository[models.Task]
	Comments repositories.Repository[models.Comment]
}

func New(db *gorm.DB) Service {
	return Service{Users: repositories.New[models.User](db), Projects: repositories.New[models.Project](db), Tasks: repositories.New[models.Task](db), Comments: repositories.New[models.Comment](db)}
}

func (s Service) Register(name, email, password string) (models.User, error) {
	if strings.TrimSpace(name) == "" || len(password) < 8 {
		return models.User{}, ErrInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}
	user := models.User{Name: strings.TrimSpace(name), Email: strings.ToLower(strings.TrimSpace(email)), PasswordHash: string(hash), Role: "user"}
	return user, s.Users.Create(&user)
}

func (s Service) Authenticate(email, password string) (models.User, error) {
	var user models.User
	err := s.Users.DB.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return models.User{}, ErrUnauthorized
	}
	return user, nil
}

func (s Service) DeleteUser(id uint, actorID uint, admin bool) error {
	if id != actorID && !admin {
		return ErrUnauthorized
	}
	return s.Users.Delete(id, 0, true)
}

func (s Service) CanAccessProject(projectID, ownerID uint, admin bool) error {
	_, err := s.Projects.Get(projectID, ownerID, admin)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gorm.ErrRecordNotFound
	}
	return err
}

func (s Service) CanAccessTask(taskID, ownerID uint, admin bool) error {
	_, err := s.Tasks.Get(taskID, ownerID, admin)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gorm.ErrRecordNotFound
	}
	return err
}
