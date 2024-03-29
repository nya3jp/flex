FROM golang:1.16 as builder

WORKDIR /build

# Install dependencies first for caching.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . ./
RUN CGO_ENABLED=0 go install github.com/nya3jp/flex/cmd/flexlet

FROM ubuntu:latest

WORKDIR /app
COPY --from=builder /go/bin/flexlet .
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

RUN useradd -M app
RUN mkdir -p -m 700 /work && chown -R app:app /work
ENV HOME=/app
USER app

ENTRYPOINT ["./flexlet", "--storedir", "/work"]
