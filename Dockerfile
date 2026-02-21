# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
COPY vendor/ vendor/

# Copy source and build
COPY . .
RUN go build -mod=vendor -o /app/bytebattle ./cmd/bytebattle

# Install migrate CLI
ARG MIGRATE_VERSION=v4.19.1
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@${MIGRATE_VERSION}

# ── Runtime stage ────────────────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bytebattle .
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY migrations/ ./migrations/

EXPOSE 8080

ENTRYPOINT ["./bytebattle"]
