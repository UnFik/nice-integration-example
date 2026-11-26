# syntax=docker/dockerfile:1

# The Go modules, the NICE SDK included, are downloaded from the module proxy, so the image builds without the nice
# repository. The build context is agent/.
FROM golang:1.27 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agen ./cmd/agent \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/agen /agen
COPY --from=build --chown=nonroot:nonroot /out/data /data
USER nonroot:nonroot
# The default 127.0.0.1:8083 of make run is unreachable from outside a container.
ENV LISTEN_ADDR=0.0.0.0:8083 JWKS_FILE=/data/jwks.json
EXPOSE 8083
ENTRYPOINT ["/agen"]
