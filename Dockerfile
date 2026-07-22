# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# go-fitz uses purego (no cgo); a static binary keeps the image tiny.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /kova ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /kova /usr/local/bin/kova
EXPOSE 8080
ENV KOVA_ADDR=:8080
ENTRYPOINT ["kova"]
