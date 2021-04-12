############# builder
FROM golang:1.16.3 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-azure
COPY . .
RUN make install

############# base
FROM alpine:3.13.4 AS base

############# gardener-extension-provider-azure
FROM base AS gardener-extension-provider-azure

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-azure /gardener-extension-provider-azure
ENTRYPOINT ["/gardener-extension-provider-azure"]

############# gardener-extension-admission-azure
FROM base as gardener-extension-admission-azure

COPY --from=builder /go/bin/gardener-extension-admission-azure /gardener-extension-admission-azure
ENTRYPOINT ["/gardener-extension-admission-azure"]
