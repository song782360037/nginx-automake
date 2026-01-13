FROM golang:1.22-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /nginx-automake .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
  build-essential \
  ca-certificates \
  curl \
  git \
  libpcre3-dev \
  libssl-dev \
  pkg-config \
  zlib1g-dev \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /nginx-automake /usr/local/bin/nginx-automake
COPY modules /app/modules
ENV PORT=8080 \
  MODULES_DIR=/app/modules \
  WORKDIR=/tmp/nginx-build
EXPOSE 8080
ENTRYPOINT ["nginx-automake"]
