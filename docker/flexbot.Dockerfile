FROM golang:1.16 as builder

WORKDIR /build

# Install dependencies first for caching.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . ./
RUN CGO_ENABLED=0 go install github.com/nya3jp/flex/cmd/flexbot

FROM debian:latest

WORKDIR /app
COPY --from=builder /go/bin/flexbot .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

RUN useradd -M app
ENV HOME=/app
USER app

ENTRYPOINT ["./flexbot"]
