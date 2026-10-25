package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/quan/task-management-api/internal/models"
	"github.com/quan/task-management-api/internal/repositories"
	"github.com/quan/task-management-api/internal/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Controller[T any] struct {
	Repo     repositories.Repository[T]
	Cache    *redis.Client
	TTL      time.Duration
	Owner    func(*T, uint)
	GetOwner func(T) uint
	Fields   map[string]string
	Validate func(T, bool) error
}

func (ctl Controller[T]) Register(r *gin.RouterGroup, path string) {
	r.GET(path, ctl.list)
	r.POST(path, ctl.create)
	r.GET(path+"/:id", ctl.get)
	r.PUT(path+"/:id", ctl.update)
	r.DELETE(path+"/:id", ctl.delete)
}

func (ctl Controller[T]) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	filters := map[string]any{}
	for key, column := range ctl.Fields {
		if value := c.Query(key); value != "" {
			if column == "project_id" || column == "task_id" {
				parsed, err := strconv.ParseUint(value, 10, 64)
				if err != nil || parsed == 0 {
					writeError(c, services.ErrInvalid)
					return
				}
				filters[column] = parsed
			} else {
				filters[column] = value
			}
		}
	}
	order := "created_at desc"
	if field := c.Query("sort"); field != "" {
		if column, ok := ctl.Fields[field]; ok {
			order = column
			if strings.EqualFold(c.Query("order"), "desc") {
				order += " desc"
			} else {
				order += " asc"
			}
		}
	}
	cacheKey := fmt.Sprintf("%s:%d:%s:%s", c.FullPath(), c.GetUint("user_id"), c.GetString("role"), c.Request.URL.RawQuery)
	if ctl.Cache != nil {
		if data, err := ctl.Cache.Get(c, cacheKey).Bytes(); err == nil {
			c.Data(http.StatusOK, "application/json; charset=utf-8", data)
			return
		}
	}
	items, total, err := ctl.Repo.List(c.GetUint("user_id"), c.GetString("role") == "admin", (page-1)*size, size, filters, order)
	if err != nil {
		writeError(c, err)
		return
	}
	payload := gin.H{"data": items, "meta": gin.H{"page": page, "page_size": size, "total": total}}
	if ctl.Cache != nil {
		if data, err := json.Marshal(payload); err == nil {
			_ = ctl.Cache.Set(c, cacheKey, data, ctl.TTL).Err()
		}
	}
	c.JSON(http.StatusOK, payload)
}

func (ctl Controller[T]) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := ctl.Repo.Get(id, c.GetUint("user_id"), c.GetString("role") == "admin")
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (ctl Controller[T]) create(c *gin.Context) {
	var item T
	if err := c.ShouldBindJSON(&item); err != nil {
		writeError(c, services.ErrInvalid)
		return
	}
	if ctl.Owner != nil {
		ctl.Owner(&item, c.GetUint("user_id"))
	}
	if ctl.Validate != nil {
		if err := ctl.Validate(item, c.GetString("role") == "admin"); err != nil {
			writeError(c, err)
			return
		}
	}
	if err := ctl.Repo.Create(&item); err != nil {
		writeError(c, err)
		return
	}
	ctl.invalidate(c)
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func (ctl Controller[T]) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input T
	if err := c.ShouldBindJSON(&input); err != nil {
		writeError(c, services.ErrInvalid)
		return
	}
	if ctl.Owner != nil {
		ownerID := c.GetUint("user_id")
		if c.GetString("role") == "admin" {
			current, err := ctl.Repo.Get(id, ownerID, true)
			if err != nil {
				writeError(c, err)
				return
			}
			if ctl.GetOwner != nil {
				ownerID = ctl.GetOwner(current)
			}
		}
		ctl.Owner(&input, ownerID)
	}
	if ctl.Validate != nil {
		if err := ctl.Validate(input, c.GetString("role") == "admin"); err != nil {
			writeError(c, err)
			return
		}
	}
	if err := ctl.Repo.Update(&input, id, c.GetUint("user_id"), c.GetString("role") == "admin"); err != nil {
		writeError(c, err)
		return
	}
	item, err := ctl.Repo.Get(id, c.GetUint("user_id"), c.GetString("role") == "admin")
	if err != nil {
		writeError(c, err)
		return
	}
	ctl.invalidate(c)
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func (ctl Controller[T]) delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := ctl.Repo.Delete(id, c.GetUint("user_id"), c.GetString("role") == "admin"); err != nil {
		writeError(c, err)
		return
	}
	ctl.invalidate(c)
	c.Status(http.StatusNoContent)
}

func (ctl Controller[T]) invalidate(c *gin.Context) {
	if ctl.Cache == nil {
		return
	}
	resourcePath := strings.TrimSuffix(strings.Split(c.FullPath(), "/:")[0], "/")
	var cursor uint64
	for {
		keys, next, err := ctl.Cache.Scan(c, cursor, resourcePath+":*", 100).Result()
		if err != nil {
			return
		}
		if len(keys) > 0 {
			_ = ctl.Cache.Del(c, keys...).Err()
		}
		cursor = next
		if cursor == 0 {
			return
		}
	}
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(c, services.ErrInvalid)
		return 0, false
	}
	return uint(id), true
}

var validate = validator.New()

func validateItem[T any](item T) error {
	if err := validate.Struct(item); err != nil {
		return services.ErrInvalid
	}
	return nil
}

func writeError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "an unexpected error occurred"
	switch {
	case errors.Is(err, services.ErrInvalid):
		status, code, message = http.StatusBadRequest, "invalid_input", "request body or parameters are invalid"
	case errors.Is(err, services.ErrUnauthorized):
		status, code, message = http.StatusUnauthorized, "unauthorized", "email or password is incorrect"
	case errors.Is(err, gorm.ErrRecordNotFound):
		status, code, message = http.StatusNotFound, "not_found", "resource not found"
	case errors.Is(err, gorm.ErrDuplicatedKey):
		status, code, message = http.StatusConflict, "conflict", "resource already exists"
	case err != nil && (strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique constraint")):
		status, code, message = http.StatusConflict, "conflict", "resource already exists"
	case errors.Is(err, gorm.ErrInvalidData):
		status, code, message = http.StatusBadRequest, "invalid_input", "request data is invalid"
	}
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func controllerSet(db *gorm.DB, cache *redis.Client, ttl time.Duration) []struct {
	Path     string
	Register func(*gin.RouterGroup)
} {
	service := services.New(db)
	project := Controller[models.Project]{Repo: repositories.New[models.Project](db), Cache: cache, TTL: ttl, Owner: func(v *models.Project, id uint) { v.OwnerID = id }, GetOwner: func(v models.Project) uint { return v.OwnerID }, Fields: map[string]string{"name": "name"}, Validate: func(v models.Project, _ bool) error { return validateItem(v) }}
	task := Controller[models.Task]{Repo: repositories.New[models.Task](db), Cache: cache, TTL: ttl, Owner: func(v *models.Task, id uint) { v.OwnerID = id }, GetOwner: func(v models.Task) uint { return v.OwnerID }, Fields: map[string]string{"status": "status", "priority": "priority", "project_id": "project_id"}, Validate: func(v models.Task, admin bool) error {
		if err := validateItem(v); err != nil {
			return err
		}
		return service.CanAccessProject(v.ProjectID, v.OwnerID, admin)
	}}
	comment := Controller[models.Comment]{Repo: repositories.New[models.Comment](db), Cache: cache, TTL: ttl, Owner: func(v *models.Comment, id uint) { v.OwnerID = id }, GetOwner: func(v models.Comment) uint { return v.OwnerID }, Fields: map[string]string{"task_id": "task_id"}, Validate: func(v models.Comment, admin bool) error {
		if err := validateItem(v); err != nil {
			return err
		}
		return service.CanAccessTask(v.TaskID, v.OwnerID, admin)
	}}
	return []struct {
		Path     string
		Register func(*gin.RouterGroup)
	}{
		{"/projects", func(r *gin.RouterGroup) { project.Register(r, "") }},
		{"/tasks", func(r *gin.RouterGroup) { task.Register(r, "") }},
		{"/comments", func(r *gin.RouterGroup) { comment.Register(r, "") }},
	}
}

func Health() gin.HandlerFunc {
	return func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}}) }
}
