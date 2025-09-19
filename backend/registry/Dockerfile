FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/registry ./cmd/registry
FROM alpine:3.20
COPY --from=build /out/registry /usr/local/bin/registry
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/registry"]
