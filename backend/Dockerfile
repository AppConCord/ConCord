# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build

RUN apk add --no-cache build-base
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/concord-api ./cmd/api

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -g 1000 concord \
    && adduser -D -u 1000 -G concord concord \
    && mkdir -p /var/lib/concord \
    && chown -R concord:concord /var/lib/concord
COPY --from=build /out/concord-api /usr/local/bin/concord-api
USER concord
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/concord-api"]
