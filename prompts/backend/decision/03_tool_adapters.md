Implement internal/tools:

- graph.go: GraphQuery(cypher string, params map[string]any, limit int) ([]map[string]any, error)
- cve.go: Lookup(hostID string, pkgs []string) ([]map[string]any, error)  // stub OK
- risk.go: HostRisk(hostID string) (struct{ FiveXX float64; RecentFindings int }, error) // stub OK
- policy.go: Compile(intent map[string]any) (artifacts []map[string]any, error) // return nft/cilium stubs
- ebpf.go: Suggest(templateHint string, context map[string]any) (map[string]any, error) // return {template, params}
- registry.go: SignAndStore(artifact map[string]any, meta map[string]any) (id string, sig string, error) // stub
- nats.go: PublishPlanProposed(plan model.Plan) error

These are pure functions with injected configs; no global state.
