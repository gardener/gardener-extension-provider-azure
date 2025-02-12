############# builder
FROM golang:1.23.6 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-azure

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG EFFECTIVE_VERSION

RUN make install EFFECTIVE_VERSION=$EFFECTIVE_VERSION

############# base
FROM gcr.io/distroless/static-debian11:nonroot AS base

############# gardener-extension-provider-azure
FROM base AS gardener-extension-provider-azure
WORKDIR /

COPY --from=builder /go/bin/gardener-extension-provider-azure /gardener-extension-provider-azure
ENTRYPOINT ["/gardener-extension-provider-azure"]

############# gardener-extension-admission-azure
FROM base AS gardener-extension-admission-azure
WORKDIR /

COPY --from=builder /go/bin/gardener-extension-admission-azure /gardener-extension-admission-azure
ENTRYPOINT ["/gardener-extension-admission-azure"]
