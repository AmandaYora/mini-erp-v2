# Rule: Frontend (React)

Applies to: `apps/web/**`.

- React 19 + Tailwind 4, react-router-dom with lazy-loaded routes/pages.
- State: Zustand (global in `shared/stores`, module state in `modules/<m>/stores`).
- Validation: Zod schemas in `modules/<m>/schemas`. Prefer `z.infer` over manual form types.
- HTTP: all requests go through `shared/services/http-client.ts` (one Axios instance).
- Use the `@/*` import alias. Centralize theme color in `src/theme/`.
- Shared UI components must be domain-agnostic. Use the `frontend-design` skill for UI; if it isn't installed, install it first from https://github.com/anthropics/skills/tree/main/skills/frontend-design (do not skip it).
