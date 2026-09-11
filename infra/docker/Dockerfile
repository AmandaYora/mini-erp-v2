# Multi-stage build: static frontend + Go API in one container.
FROM node:20-alpine AS web
WORKDIR /repo
COPY package.json ./
COPY packages ./packages
COPY apps/web ./apps/web
RUN npm install
RUN npm run build -w apps/web

FROM golang:1.22-alpine AS api
WORKDIR /src
COPY apps/api ./apps/api
WORKDIR /src/apps/api
RUN go build -o /out/api ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=api /out/api ./api
COPY --from=web /repo/apps/web/dist ./public
ENV APP_PORT=8080
ENV PUBLIC_DIR=/app/public
EXPOSE 8080
CMD ["./api"]
