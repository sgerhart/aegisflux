Create a Go module in backend/ingest (go 1.21). 
Implement a gRPC server for PostEvents(stream Event) using protos/ingest.proto:
- cmd/ingest/main.go initializes logger, reads env (INGEST_GRPC_ADDR, INGEST_HTTP_ADDR, NATS_URL), and starts both servers (gRPC + /healthz HTTP).
- internal/server/grpc.go registers Ingest service; for each incoming Event, call a Validator then Publisher.
- Use zerolog or slog for JSON logs.
- Generate proto stubs (place under backend/common/gen or local package; you choose a clean layout).
- Provide a README snippet for running locally (go run cmd/ingest/main.go).

Do not implement validation or NATS yet; stub interfaces:
type Validator interface { ValidateEvent(ctx context.Context, e *ingest.Event) error }
type Publisher interface { PublishEvent(ctx context.Context, e *ingest.Event) error }

Return Ack{ok:true} when stream closes successfully.
