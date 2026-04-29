# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

山大政管学院校友平台一期 MPA 试点项目，提供校友数据管理、校友查询、个人资料维护、数据大屏和管理员账号管理能力。

**Tech Stack:**
- Backend: Go 1.26, Gin, GORM, MySQL 8.0, Redis, Viper, zap
- Frontend: React 18, Vite, TypeScript, Ant Design, axios, Zustand, ECharts
- Local orchestration: docker-compose.yml

## Commands

**Backend (from `server/`):**
```bash
go mod tidy
go test ./...
go run ./cmd/api              # Start API server
go test -v ./internal/service # Run specific package tests
```

**Frontend (from `web/`):**
```bash
npm install
npm run dev    # Dev server at http://127.0.0.1:5173
npm run build  # TypeScript check + Vite build
```

**Full stack:**
```bash
docker compose up --build     # MySQL:3307, Redis:6379, API:8080, Web:80
make gendb                    # Regenerate GORM models from DB schema
```

**Health check:**
```bash
curl http://127.0.0.1:8080/api/v1/health/live
curl http://127.0.0.1:8080/api/v1/health/ready
```

## Architecture

**Backend (`server/`):**
- `cmd/api/main.go` — Entry point: config, logger, DB, Redis, HTTP server lifecycle
- `internal/router/` — Route registration, dependency injection at `/api/v1`
- `internal/handler/` — HTTP request binding, status codes, error mapping
- `internal/service/` — Business logic
- `internal/repository/` — Data access layer
- `internal/model/*.gen.go`, `internal/query/*.gen.go` — GORM Gen generated code (regenerate after schema changes)
- `internal/config/` — Viper config from env vars or `server/.env`
- `internal/response/` — Unified response `{ code, message, data }`

**Frontend (`web/`):**
- `src/api/` — axios request wrapper, business APIs
- `src/router/` — Routes and `RequireAuth` guards
- `src/store/` — Zustand state (authStore manages login state)
- `src/pages/` — Page components
- Roles: `alumni`, `admin`, `super_admin`

## Conventions

**Backend:**
- Maintain layers: `handler -> service -> repository -> model`. No DB details in handler, no HTTP specifics in service.
- API responses use `internal/response` unified structure, success `code=0`.
- Config via Viper from env vars or `server/.env`. Never commit real `.env` or production secrets.
- DB and Redis default to disabled; ensure service runs with `DB_ENABLED=false`, `REDIS_ENABLED=false`.
- Passwords only stored as hashes. Auth changes must not leak account existence to users.
- `internal/model/*.gen.go` and `internal/query/*.gen.go` are generated — run `make gendb` after schema changes.
- Run `gofmt` and `go test ./...` before committing backend changes.

**Frontend:**
- API requests via `src/api/http.ts` `request<T>`, compatible with backend response envelope.
- New interface types go in `src/types/`, not inline in pages.
- Login state via `src/store/authStore.ts`, never direct散落读写.
- UI text in Chinese, consistent with MPA 校友平台 context.
- Run `npm run build` before committing frontend changes.

**Database:**
- Init SQL in `server/migrations/001_init_schema.sql`
- Default super admin: `admin` / `Admin@123456` / `super_admin` (change in production!)
- GORM models generated via `make gendb` using `server/gen.yml`

## Permissions

When touching user data, alumni profiles, admin permissions, or audit logs:
- 游客: cannot view personal info
- 校友: read-only on others' profiles
- 管理员: can maintain
- 超级管理员: can manage accounts

## Scope

One-phase MPA pilot only. Do not introduce events, CMS, AI, payments, or modules not in `docs/` scope.
