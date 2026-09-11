# ADR-0002: Modular monolith backend

## Status
Accepted

## Context
We want clean module boundaries without the operational cost of microservices.

## Decision
Backend is a modular monolith. Each module exposes only `contracts/`. No cross-module
service/repository/domain imports, no cross-module DB joins or foreign keys.

## Consequences
Modules stay decoupled and could be extracted later if ever needed.
