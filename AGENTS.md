# Agent Notes

Keep changes small and aligned with the existing Go/Packer plugin patterns.

## Generated Files

- Run `make generate` after changing builder config, docs, or docs partials.
- Commit generated drift, especially `.web-docs/components/...`, when docs change.
- `make check-generate` is the CI guard for missed generated output.
- Do not commit local build/tool artifacts such as `.docs/`, `.tools/`, or `packer-plugin-verda`.

## Local Checks

CI runs these checks:

```sh
make tidy-check
make check-fmt
make check-generate
make test
make plugin-check
make lint
```

Inside sandboxed runs, use a writable Go cache when needed. Pick an OS-appropriate temp/cache path:

```sh
GOCACHE=<writable-temp-dir>/packer-plugin-verda-go-build make tidy-check
GOCACHE=<writable-temp-dir>/packer-plugin-verda-go-build make test
GOCACHE=<writable-temp-dir>/packer-plugin-verda-go-build make plugin-check
```

`make check-generate` and `make lint` may need network access the first time because they install pinned tools into `.tools/bin`.
