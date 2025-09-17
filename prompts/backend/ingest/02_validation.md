Implement JSON Schema validation for Event using schemas/Event.json.
- internal/validate/schema.go: load schemas/Event.json on startup, compile, and validate each Event by converting it to a map[string]any.
- Use github.com/santhosh-tekuri/jsonschema/v5 or similar strict validator.
- internal/validate/schema_test.go: unit tests for success and failure (missing ts, missing host_id, wrong event_type type).
- Wire Validator into the gRPC server; on invalid event return an InvalidArgument status WITH a clear message (do not crash the stream).
- Ensure validator is concurrency-safe (precompiled schema).
