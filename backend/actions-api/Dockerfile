FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/actions-api ./cmd/actions-api
FROM alpine:3.20
COPY --from=build /out/actions-api /usr/local/bin/actions-api
EXPOSE 8083
ENTRYPOINT ["/usr/local/bin/actions-api"]
