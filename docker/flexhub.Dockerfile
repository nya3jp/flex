FROM golang:1.16 as go

WORKDIR /build

# Install dependencies first for caching.
COPY go.mod go.sum ./
RUN go mod download

# Build a static binary.
COPY . ./
RUN CGO_ENABLED=0 go install github.com/nya3jp/flex/cmd/flexhub

FROM node:16 as node

WORKDIR /build

# Build a bundle.
COPY js ./
RUN cd client && npm install && npm run build
RUN cd dashboard && npm install && npm run build

FROM ubuntu:latest

WORKDIR /app
COPY --from=go /go/bin/flexhub .
COPY --from=go /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=node /build/dashboard/build ./web

RUN useradd -M app
ENV HOME=/app
USER app

ENTRYPOINT ["./flexhub"]
