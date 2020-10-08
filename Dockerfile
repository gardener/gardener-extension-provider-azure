############# builder
FROM golang:1.14.9 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-azure
COPY . .
RUN make install

############# base
FROM alpine:3.12.0 AS base

############# gardener-extension-provider-azure
FROM base AS gardener-extension-provider-azure

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-azure /gardener-extension-provider-azure
ENTRYPOINT ["/gardener-extension-provider-azure"]

############# gardener-extension-validator-azure
FROM base AS gardener-extension-validator-azure

COPY --from=builder /go/bin/gardener-extension-validator-azure /gardener-extension-validator-azure
ENTRYPOINT ["/gardener-extension-validator-azure"]
