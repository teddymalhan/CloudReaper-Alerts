# Contributing

Thanks for considering a contribution to CloudReaper Alerts.

## Getting started

```bash
git clone git@github.com:teddymalhan/CloudReaper-Alerts.git
cd CloudReaper-Alerts
go test ./...
```

See the [README](README.md) for build, run, and architecture details.

## Workflow

1. Fork the repo and create a branch off `main`.
2. Make your change, with tests for new behavior.
3. Run `go test ./...` and `make build` (or `make package`) locally.
4. Open a pull request describing the change and why it's needed.

## Code style

- Run `gofmt` (or `go fmt ./...`) before committing.
- Keep functions small and table-driven tests where it fits the existing style.
- Match the conventions already used in the package you're editing.

## Commit messages

Use a short imperative subject line, optionally prefixed with a type such as
`fix:`, `feat:`, `build:`, or `ci:`, consistent with the existing history
(`git log --oneline`).

## Reporting bugs and requesting features

Please use the issue templates when opening an issue. Include reproduction
steps, expected vs actual behavior, and relevant logs or `report.json`
output where applicable.

## Security issues

Do not open a public issue for security vulnerabilities. See
[SECURITY.md](SECURITY.md) for how to report them privately.
