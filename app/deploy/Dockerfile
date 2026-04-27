# Multi-service build: pass --build-arg SERVICE=gateway/scheduler/...
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-alpine AS build
ARG SERVICE
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/app /app/app
COPY configs /app/configs
USER nonroot:nonroot
ENTRYPOINT ["/app/app"]
