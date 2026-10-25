package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name         string `json:"name" gorm:"size:120;not null" validate:"required,min=2,max=120"`
	Email        string `json:"email" gorm:"size:255;uniqueIndex;not null" validate:"required,email,max=255"`
	PasswordHash string `json:"-" gorm:"not null"`
	Role         string `json:"role" gorm:"size:20;not null;default:user"`
}

type Project struct {
	gorm.Model
	Name        string `json:"name" gorm:"size:160;not null" validate:"required,min=2,max=160"`
	Description string `json:"description" gorm:"type:text" validate:"max=2000"`
	OwnerID     uint   `json:"owner_id" gorm:"not null;index"`
}

type Task struct {
	gorm.Model
	Title       string `json:"title" gorm:"size:200;not null" validate:"required,min=2,max=200"`
	Description string `json:"description" gorm:"type:text" validate:"max=4000"`
	Status      string `json:"status" gorm:"size:30;not null;default:todo;index" validate:"omitempty,oneof=todo in_progress done blocked"`
	Priority    string `json:"priority" gorm:"size:20;not null;default:medium" validate:"omitempty,oneof=low medium high urgent"`
	ProjectID   uint   `json:"project_id" gorm:"not null;index"`
	OwnerID     uint   `json:"owner_id" gorm:"not null;index"`
}

type Comment struct {
	gorm.Model
	Body    string `json:"body" gorm:"type:text;not null" validate:"required,min=1,max=4000"`
	TaskID  uint   `json:"task_id" gorm:"not null;index"`
	OwnerID uint   `json:"owner_id" gorm:"not null;index"`
}
