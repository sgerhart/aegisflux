Implement internal/store/fs.go:
- Put(id, tarZstBytes, metadata map) error
- Get(id) (metadata, tarZstBytes, error)
- List() []ArtifactSummary
