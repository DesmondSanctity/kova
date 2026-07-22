# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# go-fitz needs MuPDF: with cgo it links the bundled static MuPDF into the binary
# (no runtime libmupdf.so). -buildmode=pie satisfies the Unikraft elfloader.
RUN CGO_ENABLED=1 GOOS=linux go build -buildmode=pie -trimpath -ldflags="-s -w" -o /kova ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /kova /usr/local/bin/kova
EXPOSE 8080
ENV KOVA_ADDR=:8080
ENTRYPOINT ["kova"]
