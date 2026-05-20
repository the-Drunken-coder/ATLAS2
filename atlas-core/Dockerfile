ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-bookworm AS build-base

WORKDIR /src/atlas-core
COPY atlas-core/go.mod atlas-core/go.sum ./
COPY atlas-protocol/packages/go /src/atlas-protocol/packages/go
RUN go mod download
COPY atlas-core/. .

FROM build-base AS build-datastorage
RUN CGO_ENABLED=0 go build -o /atlas-datastorage ./services/datastorage/cmd/atlas-datastorage

FROM build-base AS build-functions
RUN CGO_ENABLED=0 go build -o /atlas-functions ./services/functions/cmd/atlas-functions

FROM build-base AS build-fusion
RUN CGO_ENABLED=0 go build -o /atlas-fusion ./services/fusion/cmd/atlas-fusion

FROM debian:12-slim AS runtime-datastorage-files
RUN mkdir -p /var/lib/atlas-datastorage/objects && chown -R 65532:65532 /var/lib/atlas-datastorage

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot AS atlas-datastorage
COPY --from=runtime-datastorage-files --chown=nonroot:nonroot /var/lib/atlas-datastorage /var/lib/atlas-datastorage
COPY --from=build-datastorage --chown=nonroot:nonroot /atlas-datastorage /atlas-datastorage
ENTRYPOINT ["/atlas-datastorage"]

FROM debian:12-slim AS runtime-functions-files
RUN mkdir -p /var/lib/atlas-functions && chown -R 65532:65532 /var/lib/atlas-functions

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot AS atlas-functions
COPY --from=runtime-functions-files --chown=nonroot:nonroot /var/lib/atlas-functions /var/lib/atlas-functions
COPY --from=build-functions --chown=nonroot:nonroot /atlas-functions /atlas-functions
ENTRYPOINT ["/atlas-functions"]

FROM debian:12-slim AS runtime-fusion-files
RUN mkdir -p /var/lib/atlas-fusion && chown -R 65532:65532 /var/lib/atlas-fusion

FROM --platform=${TARGETOS}/${TARGETARCH} gcr.io/distroless/static-debian12:nonroot AS atlas-fusion
COPY --from=runtime-fusion-files --chown=nonroot:nonroot /var/lib/atlas-fusion /var/lib/atlas-fusion
COPY --from=build-fusion --chown=nonroot:nonroot /atlas-fusion /atlas-fusion
ENTRYPOINT ["/atlas-fusion"]
