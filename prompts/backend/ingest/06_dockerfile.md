Create a minimal Dockerfile for backend/ingest:
- Use a multi-stage build with golang:1.21-alpine and distroless/static as final (or scratch) 
- Expose 50051 and 9090
- CMD runs the server reading envs
