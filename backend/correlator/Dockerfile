FROM golang:1.21-alpine AS build
WORKDIR /src
COPY . .
RUN go mod tidy && go build -o /out/correlator ./cmd/correlator

FROM alpine:3.20
COPY --from=build /out/correlator /usr/local/bin/correlator
EXPOSE 8085
ENTRYPOINT ["/usr/local/bin/correlator"]
