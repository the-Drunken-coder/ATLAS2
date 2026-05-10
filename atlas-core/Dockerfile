ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /atlas-core ./cmd/atlas-core

FROM --platform=${TARGETOS}/${TARGETARCH} node:24-bookworm-slim

RUN mkdir -p /var/lib/atlas-core/objects /opt/atlas-protocol \
    && chown -R node:node /var/lib/atlas-core /opt/atlas-protocol

COPY --from=build --chown=node:node /atlas-core /usr/local/bin/atlas-core
COPY --chown=node:node protocol/ /opt/atlas-protocol/

USER node

ENTRYPOINT ["atlas-core"]
