package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/quan/task-management-api/configs"
	"github.com/quan/task-management-api/internal/middlewares"
	"github.com/quan/task-management-api/internal/models"
	"github.com/quan/task-management-api/internal/repositories"
	"github.com/quan/task-management-api/internal/services"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type API struct {
	Config configs.Config
	DB     *gorm.DB
	Cache  *redis.Client
}

func (api API) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middlewares.RequestLogger(), cors.New(cors.Config{
		AllowOrigins: strings.Split(api.Config.CorsOrigins, ","), AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Authorization"}, AllowCredentials: true, MaxAge: 12 * time.Hour,
	}), middlewares.RateLimit(api.Config.RateLimit))
	r.GET("/health", Health())
	r.GET("/api/v1/health", Health())
	r.StaticFile("/docs/openapi.yaml", "docs/openapi.yaml")
	r.GET("/swagger", swaggerUI())
	servicesLayer := services.New(api.DB)
	api.authRoutes(r, servicesLayer)
	protected := r.Group("/api/v1")
	protected.Use(middlewares.Auth(api.Config.JWTSecret))
	api.userRoutes(protected, servicesLayer)
	for _, endpoint := range controllerSet(api.DB, api.Cache, time.Duration(api.Config.CacheTTL)*time.Second) {
		if endpoint.Path != "/users" {
			endpoint.Register(protected.Group(endpoint.Path))
		}
	}
	return r
}

type authInput struct {
	Name     string `json:"name" validate:"required,min=2,max=120"`
	Email    string `json:"email" validate:"required,email,max=255"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

func (api API) authRoutes(r *gin.Engine, service services.Service) {
	r.POST("/api/v1/auth/register", func(c *gin.Context) {
		var input authInput
		if c.ShouldBindJSON(&input) != nil || validate.Struct(input) != nil {
			writeError(c, services.ErrInvalid)
			return
		}
		user, err := service.Register(input.Name, input.Email, input.Password)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"data": user})
	})
	r.POST("/api/v1/auth/login", func(c *gin.Context) {
		var input struct {
			Email    string `json:"email" validate:"required,email"`
			Password string `json:"password" validate:"required"`
		}
		if c.ShouldBindJSON(&input) != nil || validate.Struct(input) != nil {
			writeError(c, services.ErrInvalid)
			return
		}
		user, err := service.Authenticate(input.Email, input.Password)
		if err != nil {
			writeError(c, err)
			return
		}
		token, err := middlewares.Token(api.Config.JWTSecret, user.ID, user.Role)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"token": token, "token_type": "Bearer", "expires_in": 86400, "user": user}})
	})
}

func (api API) userRoutes(r *gin.RouterGroup, service services.Service) {
	repo := repositories.New[models.User](api.DB)
	admin := r.Group("/users")
	admin.Use(middlewares.Admin())
	admin.GET("", func(c *gin.Context) { listUsers(c, repo) })
	admin.POST("", func(c *gin.Context) { createUser(c, service) })
	r.GET("/users/:id", func(c *gin.Context) { getUser(c, repo) })
	r.PUT("/users/:id", func(c *gin.Context) { updateUser(c, repo) })
	r.DELETE("/users/:id", func(c *gin.Context) { deleteUser(c, service) })
}

func listUsers(c *gin.Context, repo repositories.Repository[models.User]) {
	items, total, err := repo.List(0, true, 0, 100, nil, "created_at desc")
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "meta": gin.H{"total": total}})
}

func createUser(c *gin.Context, service services.Service) {
	var input struct {
		Name     string `json:"name" validate:"required,min=2,max=120"`
		Email    string `json:"email" validate:"required,email,max=255"`
		Password string `json:"password" validate:"required,min=8,max=72"`
		Role     string `json:"role" validate:"omitempty,oneof=user admin"`
	}
	if c.ShouldBindJSON(&input) != nil || validate.Struct(input) != nil {
		writeError(c, services.ErrInvalid)
		return
	}
	user, err := service.Register(input.Name, input.Email, input.Password)
	if err != nil {
		writeError(c, err)
		return
	}
	if input.Role == "admin" {
		user.Role = "admin"
		if err := service.Users.Update(&user, user.ID, 0, true); err != nil {
			writeError(c, err)
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{"data": user})
}

func getUser(c *gin.Context, repo repositories.Repository[models.User]) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	admin := c.GetString("role") == "admin"
	if id != c.GetUint("user_id") && !admin {
		writeError(c, services.ErrUnauthorized)
		return
	}
	user, err := repo.Get(id, 0, true)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func updateUser(c *gin.Context, repo repositories.Repository[models.User]) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	admin := c.GetString("role") == "admin"
	if id != c.GetUint("user_id") && !admin {
		writeError(c, services.ErrUnauthorized)
		return
	}
	var input struct {
		Name     string `json:"name" validate:"omitempty,min=2,max=120"`
		Email    string `json:"email" validate:"omitempty,email,max=255"`
		Password string `json:"password" validate:"omitempty,min=8,max=72"`
	}
	if c.ShouldBindJSON(&input) != nil || validate.Struct(input) != nil {
		writeError(c, services.ErrInvalid)
		return
	}
	user, err := repo.Get(id, 0, true)
	if err != nil {
		writeError(c, err)
		return
	}
	if input.Name != "" {
		user.Name = input.Name
	}
	if input.Email != "" {
		user.Email = strings.ToLower(input.Email)
	}
	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(c, err)
			return
		}
		user.PasswordHash = string(hash)
	}
	if err := repo.Update(&user, id, 0, true); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": user})
}

func deleteUser(c *gin.Context, service services.Service) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := service.DeleteUser(id, c.GetUint("user_id"), c.GetString("role") == "admin"); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func swaggerUI() gin.HandlerFunc {
	return func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/docs/openapi.yaml") }
}
