- internal/rules/evaluate.go:
  * Keep hostLabels cache: hostID -> []labels (from ev.context.labels, TTL CORR_LABEL_TTL_SEC).
  * OnEvent(ev):
     - Update window and labels cache.
     - Get effective rules for host.
     - For each rule: check WHEN.AnyOf (type and regex), and optional REQUIRES_PRIOR.AnyOf using window within rule.window_seconds.
     - If match: render evidence templates (replace {field} from Event fields, args.*, context.*).
     - Dedupe: expand key_template placeholders; if key in cache within cooldown, skip.
     - Emit Finding (severity/confidence/ttl_seconds from rule); increment metrics.
- Unit tests: positive, outside window negative, non-matching binary negative, dedupe prevents duplicates.
