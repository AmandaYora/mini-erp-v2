# ADR-0003: Frontend standard

## Status
Accepted

## Context
We want a consistent, simple frontend stack.

## Decision
React 19 + Tailwind 4, react-router-dom (lazy routes), Zustand, Zod, Axios, `@/*` alias,
centralized theme. No TanStack libraries by default.

## Consequences
Predictable structure; simple data patterns until advanced needs are proven.
