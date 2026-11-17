# syntax=docker/dockerfile:1

# The backend serves the built UI from its own origin, so the image builds both. The module uses
# `replace ../../../nice`, so the build also needs the nice repo as the named context "nice", and the UI
# as the named context "web" (compose.yml passes both with additional_contexts). Only go.mod, go.sum and
# pkg/ of nice are copied.
FROM node:22-alpine AS ui
WORKDIR /web
COPY --from=web package.json package-lock.json ./
RUN npm ci
COPY --from=web . .
RUN npm run build

FROM golang:1.27 AS build
WORKDIR /src/example/sharedservice-oauth/backend
COPY --from=nice go.mod go.sum /src/nice/
COPY --from=nice pkg /src/nice/pkg
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/inventaris ./cmd/inventaris \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/inventaris /inventaris
COPY --from=build --chown=nonroot:nonroot /out/data /data
COPY --from=ui /web/dist /web
USER nonroot:nonroot
ENV JWKS_FILE=/data/jwks.json WEB_DIR=/web
EXPOSE 8081
ENTRYPOINT ["/inventaris"]
