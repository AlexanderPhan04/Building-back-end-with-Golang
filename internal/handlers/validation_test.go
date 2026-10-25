package handlers

import (
	"testing"

	"github.com/quan/task-management-api/internal/models"
	"github.com/quan/task-management-api/internal/services"
	"github.com/stretchr/testify/require"
)

func TestProjectRequiresName(t *testing.T) {
	err := validateItem(models.Project{Description: "missing name"})
	require.ErrorIs(t, err, services.ErrInvalid)
}

func TestTaskRejectsUnknownStatus(t *testing.T) {
	err := validateItem(models.Task{Title: "Ship API", Status: "later"})
	require.ErrorIs(t, err, services.ErrInvalid)
}

func TestCommentRequiresBody(t *testing.T) {
	err := validateItem(models.Comment{TaskID: 7})
	require.ErrorIs(t, err, services.ErrInvalid)
}
