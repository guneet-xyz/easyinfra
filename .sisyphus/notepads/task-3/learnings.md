## Task 3: Config Types & Example YAML

### Completed
- ✅ Created `pkg/config/types.go` with 5 structs (InfraConfig, Defaults, PostRenderer, BackupConfig, AppConfig)
- ✅ All structs use `yaml:"..."` tags with `omitempty` for optional fields
- ✅ Created `pkg/config/example_infra.yaml` with all 11 apps from user's infra
- ✅ Created `pkg/config/types_test.go` with TestExampleParses and TestRoundTrip
- ✅ Both tests PASS with 11 apps verified
- ✅ No LSP diagnostics
- ✅ Evidence saved to .sisyphus/evidence/task-3-*.txt

### Key Patterns
- YAML unmarshaling uses `gopkg.in/yaml.v3` (added to go.mod)
- Pointer fields for optional nested structs (PostRenderer)
- Slice fields for arrays (Apps, ValueFiles, Args, DependsOn, PVCs)
- Round-trip test validates no field loss during marshal/unmarshal cycle

### App Order & Dependencies
- caddy (1) → registry (2) → walls (3, depends on caddy+registry)
- headlamp (4), litellm (5), openwebui (6), infisical (7)
- portainer (8), homepage (9), tailscale (10), demo (11)

### PVC Mappings Verified
- caddy: caddy-data
- registry: registry-data
- walls: walls-postgres-data
- litellm: litellm-data, litellm-postgres-data
- openwebui: openwebui-data
- infisical: infisical-postgres-data
