# Single-binary llmhub backend image.
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/llmhub ./cmd/llmhub

FROM alpine:3.20
RUN addgroup -S nonroot && adduser -S nonroot -G nonroot
WORKDIR /app
COPY --from=build /out/llmhub /app/llmhub
COPY configs /app/configs
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/llmhub"]
