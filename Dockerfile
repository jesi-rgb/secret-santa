FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=linux/x86_64 golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
COPY --from=web /web/dist ./web/dist
RUN set -xe; \
    go build \
      -buildmode=pie \
      -ldflags "-s -w -linkmode external -extldflags -static-pie" \
      -tags netgo \
      -o /secret-santa . \
    ;

FROM scratch
COPY --from=build /secret-santa /secret-santa
