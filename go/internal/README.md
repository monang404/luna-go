# internal/ — package layout

This tree mirrors the module map defined in `RENCANA_MIGRASI_GO_RUST.md`
(§2, "Pemetaan Modul → Package"). Every package below is currently an empty
placeholder (`doc.go` only) created in **SESSION-40** — no zsh logic has been
ported yet. Each package's `doc.go` states which zsh module it will replace
and which future session is responsible for filling it in.

| Go package            | zsh source                  | Ported in     |
|------------------------|------------------------------|----------------|
| `internal/env`         | `.luna/00-core/`        | SESSION-41     |
| `internal/config`      | `30-luna/00-config/`           | SESSION-41     |
| `internal/permission`  | `30-luna/06-permissions/`      | SESSION-42     |
| `internal/tools`       | `30-luna/05-tools/`            | SESSION-43/47/48 |
| `internal/llmclient`   | `30-luna/10-core/`              | SESSION-44 (blocking path; 45/46 pending) |
| `internal/agent`       | `30-luna/50-agent/`             | SESSION-49/50  |
| `internal/subagent`    | `30-luna/55-subagent/`          | SESSION-51     |
| `internal/ui`          | `30-luna/60-ui/`                | SESSION-52/53  |
| `internal/chat`        | `30-luna/20-chat/`              | SESSION-54     |
| `internal/codeproject` | `30-luna/30-code/`              | SESSION-54     |
| `internal/filepatch`   | `30-luna/35-files/`             | SESSION-54     |
| `internal/workflow`    | `30-luna/40-workflow/`          | SESSION-54     |

`.luna/10-plugins/` and `.luna/20-shell/` are interactive-shell
configuration (zinit, aliases, prompt) and are **not** being ported — they
stay the responsibility of `.zshrc`. The Go binary (`cmd/luna`) is a
standalone CLI that can still be invoked from within an ordinary zsh session
as an external command.

Rules for this directory while migration is in progress:

- Don't add real logic to a package before its assigned session starts —
  see `docs/execution_sessions/SESSION-<N>*.yaml` for scope/acceptance
  criteria per package.
- Keep package names identical to the directory name (`package config` in
  `internal/config/`, etc).
- Each new file added to a package during its porting session should carry a
  comment referencing the original zsh file(s) it replaces, for traceability
  (see `docs/MIGRATION_TRACEABILITY.md`).
