# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/parser-service ./cmd/service

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl \
    poppler-utils tesseract-ocr tesseract-ocr-rus \
    libreoffice-writer-nogui libreoffice-calc-nogui \
    unar fonts-dejavu-core fonts-liberation \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/parser-service /usr/local/bin/parser-service
ENV HTTP_ADDR=:8091
EXPOSE 8091
HEALTHCHECK --interval=15s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:8091/health || exit 1
CMD ["parser-service"]
