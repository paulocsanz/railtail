FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN CGO_ENABLED=0 go build -o railtail -ldflags="-w -s" ./.

# caixote fork: trust our self-signed headscale CA (no public domain
# available for a real Let's Encrypt cert — see the caixote repo's
# docker/headscale/config.yaml for the full story). distroless/static has
# no update-ca-certificates tooling, so merge our CA into the system
# bundle here and copy the merged bundle into the final stage.
FROM debian:bookworm-slim AS certs
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY caixote-headscale-ca.crt /usr/local/share/ca-certificates/caixote-headscale-ca.crt
RUN update-ca-certificates

FROM gcr.io/distroless/static

WORKDIR /app

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/railtail /usr/local/bin/railtail

ENTRYPOINT ["/usr/local/bin/railtail"]
