- internal/rules/override.go:
  * In-memory slice/map of Rule overrides with add/remove/list.
- internal/api/http.go:
  * GET /rules -> {files:[summary], overrides:[summary]}
  * POST /rules/overrides -> add override; return {id}
  * DELETE /rules/overrides/{id} -> remove
Wire loader snapshot + overrides into evaluate path.
