# Rule: Monorepo

Applies to: whole repository.

- Start frontend and backend separately: `npm run dev:web` and `npm run dev:api`.
- Do not introduce a combined `npm run dev` or `concurrently` unless explicitly requested.
- Keep `CLAUDE.md` a concise gateway; put detail in `knowledge/`.
- Do not add Microservices, Kubernetes, Nx, Turborepo complexity, Redis, or Memcache by default.
