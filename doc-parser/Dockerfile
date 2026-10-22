# syntax=docker/dockerfile:1
# Build from repository root: docker build -f doc-parser/Dockerfile .
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/doc-parser ./cmd/docparser

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl \
    poppler-utils tesseract-ocr tesseract-ocr-rus \
    libreoffice-writer-nogui libreoffice-calc-nogui \
    fonts-dejavu-core fonts-liberation locales \
  && sed -i 's/^# *en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen \
  && locale-gen \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/doc-parser /usr/local/bin/doc-parser
ENV HTTP_ADDR=:8094
ENV SOFFICE_PATH=/usr/bin/soffice
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8
EXPOSE 8094
HEALTHCHECK --interval=15s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:8094/health || exit 1
CMD ["doc-parser"]
