# F6-1: satu image app (frontend statis + API Go). Database tetap di host (F6-2).
# - Go 1.26.7 sesuai apps/api/go.mod (toolchain go1.26.7); GOTOOLCHAIN=local agar
#   gagal cepat bila tag image lebih tua, bukan resolve diam-diam.
# - npm ci memakai package-lock.json root (reproducible); bukan npm install.
# - ca-certificates wajib: tanpa ini panggilan HTTPS ke S3 gagal verifikasi TLS.
# - non-root 10001 + HEALTHCHECK ke /readyz (DB + direktori + versi migrasi).
# - -buildvcs=false wajib: .git dikecualikan .dockerignore sehingga stamping VCS gagal.

FROM node:20-alpine AS web
WORKDIR /repo
COPY package.json package-lock.json ./
COPY packages ./packages
COPY apps/web/package.json ./apps/web/
RUN npm ci --workspace apps/web --include-workspace-root
COPY apps/web ./apps/web
RUN npm run build -w apps/web

FROM golang:1.26.7-alpine AS api
ENV CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOTOOLCHAIN=local
WORKDIR /src/apps/api
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api ./
ARG VERSION=dev
RUN go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/server \
 && go build -trimpath -ldflags "-s -w" -o /out/seed ./cmd/seed \
 && go build -trimpath -ldflags "-s -w" -o /out/costrebuild ./cmd/costrebuild
RUN go install -tags mysql github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && addgroup -S app && adduser -S -G app -u 10001 app \
 && mkdir -p /data/storage && chown -R app:app /data
WORKDIR /app
COPY --from=api /out/api /out/seed /out/costrebuild ./
COPY --from=api /go/bin/migrate /usr/local/bin/migrate
COPY apps/api/migrations ./migrations
COPY --from=web /repo/apps/web/dist ./public
ENV APP_PORT=8080 PUBLIC_DIR=/app/public STORAGE_DIR=/data/storage
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 CMD wget -qO- http://127.0.0.1:8080/readyz >/dev/null || exit 1
ENTRYPOINT ["./api"]
