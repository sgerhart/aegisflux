Multi-stage Dockerfile:
- build with golang:1.21-alpine
- run with gcr.io/distroless/base-debian12 (or alpine)
- copy rules.d/*.yaml into image (defaults)
- expose 8082
