# mini-erp

Revamp of a single-company, multi-branch operational ERP — Go modular-monolith backend + MySQL +
React 19 frontend, generated to Dimas' monorepo standard.

**Status: project setup complete, no business module implemented yet.** See
[`docs/PRD.md`](docs/PRD.md) and [`docs/SYSTEM_DESIGN.md`](docs/SYSTEM_DESIGN.md) for scope and
the module build order, and [`knowledge/MODULE_MAP.md`](knowledge/MODULE_MAP.md) before starting
work on any module.

## Development

Run frontend and backend separately from the project root:

```bash
npm run dev:web
npm run dev:api
```

## Structure

- `apps/web` — React 19 + Tailwind 4 frontend
- `apps/api` — go modular-monolith backend
- `packages/` — shared code and API contract
- `knowledge/` — project knowledge base for Claude Code (read before editing)
- `.claude/rules/` — path-scoped technical rules
- `docs/` — PRD, system design, API contract, DB schema, deployment
- `docs/legacy-reference/` — full analysis of the previous NestJS system (21 modules, known
  issues, open questions) — required reading before designing any module
- `infra/` — Docker and nginx

## Deployment

One Docker app container serving both the static frontend and the API on port 8080.
The database runs on the host (see `docs/DEPLOYMENT.md`).
