# Architecture

The service uses a layered Go layout: Gin handlers parse and validate requests, services enforce business rules and authorization, repositories wrap GORM persistence, and models describe PostgreSQL entities. Middleware handles JWT, CORS, request logging, and rate limiting. Redis is an optional cache for list reads; database remains the source of truth. Docker Compose runs the API, PostgreSQL, and Redis. Migrations are applied by GORM AutoMigrate at startup.

```text
HTTP client -> Gin middleware -> handler -> service -> repository -> PostgreSQL
                                   |                         |
                                   +------ Redis cache -------+
```

All protected data is scoped to its owner. Admins may manage any account and resources. Passwords are bcrypt hashes; JWT signing key is supplied through environment configuration. Graceful shutdown drains the HTTP server on termination.
