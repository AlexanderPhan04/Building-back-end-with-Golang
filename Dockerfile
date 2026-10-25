FROM golang:1.21-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/task-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/task-api /app/task-api
COPY --from=build /src/docs/openapi.yaml /app/docs/openapi.yaml
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/task-api"]
