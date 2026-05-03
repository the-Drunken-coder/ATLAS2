ARG TARGETARCH=amd64
ARG TARGETOS=linux

FROM --platform=${TARGETOS}/${TARGETARCH} golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /atlas-core ./cmd/atlas-core

FROM --platform=${TARGETOS}/${TARGETARCH} alpine:3.21

RUN addgroup -g 10001 atlas && adduser -D -u 10001 -G atlas atlas

RUN mkdir -p /var/lib/atlas-core/objects && chown -R atlas:atlas /var/lib/atlas-core

COPY --from=build /atlas-core /usr/local/bin/atlas-core

USER atlas
ENTRYPOINT ["/usr/local/bin/atlas-core"]
