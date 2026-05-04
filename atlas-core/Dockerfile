ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /atlas-core ./cmd/atlas-core

FROM debian:12-slim AS runtime-files
RUN mkdir -p /var/lib/atlas-core/objects && chown -R 65532:65532 /var/lib/atlas-core

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot

# If you reuse an older volume or bind mount with root-owned paths, ensure /var/lib/atlas-core is writable by uid 65532.
COPY --from=runtime-files --chown=nonroot:nonroot /var/lib/atlas-core /var/lib/atlas-core
COPY --from=build --chown=nonroot:nonroot /atlas-core /atlas-core

ENTRYPOINT ["/atlas-core"]
