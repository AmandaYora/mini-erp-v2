---
paths:
  - "apps/web/**"
---

# Rule: Frontend (React)

- React 19 + Tailwind 4, react-router-dom with lazy-loaded routes/pages.
- State: Zustand (global in `shared/stores`, module state in `modules/<m>/stores`).
- Validation: Zod schemas in `modules/<m>/schemas`. Prefer `z.infer` over manual form types.
- HTTP: all requests go through `shared/services/http-client.ts` (one Axios instance). Show the API's
  own `message` on failure; never replace it with generic text.
- Use the `@/*` import alias. Centralize theme color in `src/theme/`: semantic token classes only, no hex,
  no raw Tailwind palette.
- Modules import UI from `shared/components/ui` and `shared/components/patterns`, never from
  `shared/components/primitives` (ESLint enforces; Design System v2, ADR-0017).
- Shared UI components must be domain-agnostic. Use the `frontend-design` skill for UI; if it isn't installed, install it first from https://github.com/anthropics/skills/tree/main/skills/frontend-design (do not skip it).

Details: `knowledge/FRONTEND_GUIDE.md`.
