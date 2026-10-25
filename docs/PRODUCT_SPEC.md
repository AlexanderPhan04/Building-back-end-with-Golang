# Product Specification

## Goal
Build a documented, containerized Go REST API for managing users, projects, tasks, and comments.

## Core requirements
- Gin HTTP API, PostgreSQL persistence through GORM, input validation, normalized JSON responses, and centralized error handling.
- User registration/login with bcrypt password hashes and JWT authentication; role-aware authorization.
- CRUD endpoints for users, projects, tasks, and comments. Users can access only their own data unless an admin role applies.
- Request logging, CORS, rate limiting, optional Redis caching, pagination/filtering/sorting, soft deletes, health checks, and graceful shutdown.
- Unit tests for at least three service/handler units, Docker Compose, multi-stage Dockerfile, CI, and API documentation.

## Scope limits
- Selected assignment topic: Task Management.
- Cloud deployment and submission report/video require the owner's deployment account and personal/student details; provide deploy-ready assets and documentation without publishing externally or inventing personal information.
