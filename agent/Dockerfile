# syntax=docker/dockerfile:1

# One image for the MCP server and the chat dashboard: the build context is agent/, so it holds backend/ and web/.
# The Go modules, the NICE SDK included, come from the module proxy, so the image builds without the nice repository.
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM golang:1.27 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agen ./cmd/agent \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/agen /agen
COPY --from=web /web/dist /web
COPY --from=build --chown=nonroot:nonroot /out/data /data
USER nonroot:nonroot
# The default 127.0.0.1:8083 of make run is unreachable from outside a container. The dashboard is served from /web.
ENV LISTEN_ADDR=0.0.0.0:8083 JWKS_FILE=/data/jwks.json WEB_DIR=/web
EXPOSE 8083
ENTRYPOINT ["/agen"]
