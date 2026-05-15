ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build

WORKDIR /src/atlas-core
COPY atlas-core/go.mod atlas-core/go.sum ./
COPY atlas-protocol/packages/go /src/atlas-protocol/packages/go
RUN go mod download
COPY atlas-core/. .
RUN CGO_ENABLED=0 go build -o /atlas-datastorage ./services/datastorage/cmd/atlas-datastorage
RUN CGO_ENABLED=0 go build -o /atlas-functions ./services/functions/cmd/atlas-functions

FROM debian:12-slim AS runtime-datastorage-files
RUN mkdir -p /var/lib/atlas-datastorage/objects && chown -R 65532:65532 /var/lib/atlas-datastorage

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot AS atlas-datastorage
COPY --from=runtime-datastorage-files --chown=nonroot:nonroot /var/lib/atlas-datastorage /var/lib/atlas-datastorage
COPY --from=build --chown=nonroot:nonroot /atlas-datastorage /atlas-datastorage
ENTRYPOINT ["/atlas-datastorage"]

FROM debian:12-slim AS runtime-functions-files
RUN mkdir -p /var/lib/atlas-functions && chown -R 65532:65532 /var/lib/atlas-functions

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot AS atlas-functions
COPY --from=runtime-functions-files --chown=nonroot:nonroot /var/lib/atlas-functions /var/lib/atlas-functions
COPY --from=build --chown=nonroot:nonroot /atlas-functions /atlas-functions
ENTRYPOINT ["/atlas-functions"]
