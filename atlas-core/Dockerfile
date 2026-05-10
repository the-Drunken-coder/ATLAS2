ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /atlas-core ./cmd/atlas-core \
    && mkdir -p /runtime/var/lib/atlas-core/objects
FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/nodejs24-debian12:nonroot

ENV ATLAS_PROTOCOL_NODE_BIN=/nodejs/bin/node \
    ATLAS_PROTOCOL_VALIDATOR_PATH=/opt/atlas-protocol/atlas-protocol-validator.mjs

COPY --from=build --chown=nonroot:nonroot /atlas-core /usr/local/bin/atlas-core
COPY --from=build --chown=nonroot:nonroot /runtime/var/lib/atlas-core /var/lib/atlas-core
COPY --chown=nonroot:nonroot protocol/ /opt/atlas-protocol/

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/atlas-core"]
