# syntax=docker/dockerfile:1

# The backend serves the built UI from its own origin, so the image builds both. The UI comes from the named
# context "web" (compose.yml passes it with additional_contexts). The Go modules, the NICE SDK included, are
# downloaded from the module proxy.
FROM node:22-alpine AS ui
WORKDIR /web
COPY --from=web package.json package-lock.json ./
RUN npm ci
COPY --from=web . .
RUN npm run build

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/nadi ./cmd/nadi \
 && mkdir /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/nadi /nadi
COPY --from=build --chown=nonroot:nonroot /out/data /data
COPY --from=ui /web/dist /web
USER nonroot:nonroot
ENV JWKS_FILE=/data/jwks.json WEB_DIR=/web
EXPOSE 8081
ENTRYPOINT ["/nadi"]
