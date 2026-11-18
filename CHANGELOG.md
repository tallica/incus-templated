# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

`caddy-config` is now `incus-templated`: it renders any Go template from
labelled Incus instances, with Caddy as one preset. Every flag, environment
variable and default path changed; there is no compatibility layer.

### Added

- `--target` renders a template per label to a file inside an instance or on
  the local filesystem, with optional `format`, `validate` and `reload`
  commands; `{staging}` and `{path}` are substituted in each. `no-format`,
  `no-validate` and `no-reload` turn a preset's step off. (by @tallica)
- `preset=caddy` gives a target the Caddy template, path and commands; any of
  them can be overridden per target. (by @tallica)
- `--config` reads targets and `templates_dir` from a YAML file, with commands
  as lists. Unknown keys are rejected, and a relative `templates_dir` is
  resolved against the file's directory. (by @tallica)
- Templates get `.Label`, `.Flags` and `.Instances`, each instance with its
  name, project, cluster member (`.Location`), label values, config, devices
  and network interfaces, plus the `json`, `join`, `split` and `required`
  funcs; `required` fails the render on an empty value. An instance that
  fails to render is left out of the file instead of holding back the rest.
  Prometheus file_sd and per-member examples are in the docs. (by @tallica)
- Every target's template is read and parsed at startup; a bad target stops
  `incus-templated` before it connects. (by @tallica)

### Changed

- The binary, command and image are `incus-templated`
  (`ghcr.io/jochumdev/incus-templated/incus-templated`). (by @tallica)
- `--caddy-instance` and `--os-path` are replaced by
  `--target <label>,preset=caddy,instance=<inst>,project=<proj>` and
  `--target <label>,preset=caddy,path=<path>`. A local target no longer
  defaults its label to `caddy`. (by @tallica)
- `--caddyfile-path` is removed; set `path=` on the target. (by @tallica)
- The `global_template=` flag is removed; to change the global block, copy the
  Caddy preset's template and pass it as `template=`. (by @tallica)
- Environment variables are renamed from `INCUS_CADDY_*` to
  `INCUS_TEMPLATED_*`, and `INCUS_CADDY_INSTANCES` / `INCUS_CADDY_OS_PATHS`
  become `INCUS_TEMPLATED_TARGETS`. `INCUS_REMOTE` and `SERVER_FINGERPRINT`
  are unchanged. (by @tallica)
- The default `--data-dir` is `/var/lib/incus-templated` and the enrolled
  certificate is named `incus-templated`, so an existing deployment enrolls
  again: provide a new trust token, or move the old data directory. (by @tallica)
- A local reload that fails, including Caddy not running, is now a failed
  deploy and is retried; it was logged and treated as success. Use
  `reload: [systemctl, try-reload-or-restart, caddy]` or `no-reload` if the
  service may be down. (by @tallica)
- A written file keeps the mode of the file it replaces, and a new file is
  `0644`. A local file was always `0600`, which other users could not read;
  one written into an instance took the staging file's mode. (by @tallica)
- A live migration to another cluster member re-renders the targets the
  instance feeds. (by @tallica)

### Fixed

- A target instance that starts while its file is being written is reloaded,
  instead of serving the previous file until the next change, e.g. Caddy
  missing routes after the first `up`. (by @tallica)
- A deployment replaced by a newer change, or stopped by shutdown, is logged at
  debug as superseded instead of as an error with a retry warning. (by @tallica)
