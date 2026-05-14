ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build

WORKDIR /src/atlas-core
COPY atlas-core/go.mod atlas-core/go.sum ./
COPY atlas-protocol/packages/go /src/atlas-protocol/packages/go
RUN go mod download
COPY atlas-core/. .
RUN CGO_ENABLED=0 go build -o /atlas-core ./cmd/atlas-core

FROM debian:12-slim AS runtime-files
RUN mkdir -p /var/lib/atlas-core/objects && chown -R 65532:65532 /var/lib/atlas-core

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot

# If you reuse an older volume or bind mount with root-owned paths, run `chown -R 65532:65532 /var/lib/atlas-core` on the host before mounting it so the distroless nonroot user can write there.
COPY --from=runtime-files --chown=nonroot:nonroot /var/lib/atlas-core /var/lib/atlas-core
COPY --from=build --chown=nonroot:nonroot /atlas-core /atlas-core

ENTRYPOINT ["/atlas-core"]
