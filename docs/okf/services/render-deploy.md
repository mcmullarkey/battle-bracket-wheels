---
type: Service
title: Render.com Deploy Config
description: Render.com web service configuration — Go build, health check, port binding
resource: render.yaml
tags: [deploy, render, config]
timestamp: 2026-06-24
pure: false
---

# Responsibility

Define Render.com deployment: build command, start command, health check, environment.

# Interface

- `render.yaml` — web service definition
- Build: `go build -o battle-bracket-wheels .`
- Start: `./battle-bracket-wheels`
- Health: `/health` endpoint
- `PORT=10000` environment

# Dependencies

- Deploys [app-architecture](/modules/app-architecture.md)
- Health check hits [http-router](/interfaces/http-router.md) `/health`

# Invariants

- Bind `0.0.0.0:$PORT` (not localhost) for Render container
- `healthCheckPath: /health`
- Free tier compatible

# Pure/Effectful

Effectful. Deployment configuration.

# Citations

- [render.yaml](render.yaml)
