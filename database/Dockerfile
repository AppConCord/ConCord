# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build

RUN apk add --no-cache build-base
WORKDIR /src/database
COPY database/go.mod database/go.sum ./
RUN go mod download
COPY database/ ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/concord-migrate ./cmd/migrate

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -g 1000 concord \
    && adduser -D -u 1000 -G concord concord \
    && mkdir -p /var/lib/concord \
    && chown -R concord:concord /var/lib/concord
WORKDIR /app
COPY --from=build /out/concord-migrate /usr/local/bin/concord-migrate
COPY database/migrations/ /app/migrations/
USER concord
ENTRYPOINT ["/usr/local/bin/concord-migrate"]
CMD ["status"]
