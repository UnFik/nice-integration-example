# syntax=docker/dockerfile:1

# The Go modules, the NICE SDK included, are downloaded from the module proxy. The UI has an image of its own
# (../web/Dockerfile), so this one serves no UI.
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
USER nonroot:nonroot
ENV JWKS_FILE=/data/jwks.json WEB_DIR=""
EXPOSE 8081
ENTRYPOINT ["/nadi"]
