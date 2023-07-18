############# builder
FROM golang:1.20.4 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-azure
COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM gcr.io/distroless/static-debian11:nonroot AS base

############# gardener-extension-provider-azure
FROM base AS gardener-extension-provider-azure
WORKDIR /

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-azure /gardener-extension-provider-azure
ENTRYPOINT ["/gardener-extension-provider-azure"]

############# gardener-extension-admission-azure
FROM base as gardener-extension-admission-azure
WORKDIR /

COPY --from=builder /go/bin/gardener-extension-admission-azure /gardener-extension-admission-azure
ENTRYPOINT ["/gardener-extension-admission-azure"]
