# S17 — Configuration Manager — Updated Plan

All 10 feedback points incorporated. Proceeding to implementation.

## Revised File Structure

```
internal/config/
  config.go              — Core types, domain structs (stable domains typed, plugins dynamic)
  sources.go             — Source enum, string representation
  errors.go              — ValidationError, ConfigError, Warning types
  defaults.go            — DefaultConfig() with sensible defaults
  env_mapper.go          — Field mapping table, ResolveEnvOverrides(), applyOverrides()
  merge.go               — YAML merge, flattenMapKeys, per-domain merge helpers
  schema_validation.go   — Structural: required, enums, types, ranges
  business_validation.go — Semantic: paths exist, mode-specific checks → warnings
  profile.go             — Immutable Profile, read-only accessors, introspection
  loader.go              — Manager interface, LoadOptions, load pipeline orchestration
```

## Key Design Changes from Feedback

1. ✅ Typed stable domains + dynamic `map[string]any` for plugins
2. ✅ Separate `Sources map[string]Source` in Profile, clean Config struct
3. ✅ Split: `schema_validation.go` + `business_validation.go`
4. ✅ Loader receives all sources via `LoadOptions` — no CLI/env knowledge
5. ✅ Plugin config: `map[string]any` per plugin
6. ✅ `env_mapper.go` with mapping table — no manual switch
7. ✅ `Manager` interface with `Load()` + `Current()`
8. ✅ Explicit load order: Defaults → System → User → Env → CLI
9. ✅ Profile is immutable after creation (unexported fields, copy-on-read)
10. ✅ SchemaVersion constant with validation hook for future migration
