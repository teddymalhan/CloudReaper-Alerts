<h1 align="center">CloudReaper Alerts</h1>
<img width="2377" height="1995" alt="bettershot_1781242034744" src="https://github.com/user-attachments/assets/c8b237fa-d84a-446a-b226-f5b48f918f04" />

<p align="center">
A self-contained <b>Go + Terraform</b> system that detects orphaned AWS resources and posts an
automatic Slack alert through an SQS-backed pipeline.
</p>


## Architecture

<img width="2314" height="2091" alt="My First Board (2)" src="https://github.com/user-attachments/assets/37944f23-6697-4b7d-ac71-f621f43f34ab" />


## Building

```bash
make package       # everything: CLIs → build/, Lambda zips → terraform/build/
make cli           # just detector + sender
make lambdas       # just notifier.zip + reactor.zip

make test          # go test ./...
make clean         # remove all build output
```

Override `LAMBDA_GOARCH` if your real Lambda arch differs from your machine (e.g. `make lambdas LAMBDA_GOARCH=amd64`).

## Releases

Releases are published to GitHub Releases automatically when a version tag is pushed:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The [release workflow](.github/workflows/release.yml) runs [GoReleaser](https://goreleaser.com) and publishes:

| Artifact | Description |
|---|---|
| `cloudreaper-alerts_detector_<version>_<os>_<arch>.tar.gz` | Detector CLI — Linux / macOS / Windows, amd64 + arm64 |
| `cloudreaper-alerts_sender_<version>_<os>_<arch>.tar.gz` | Sender CLI — same platforms |
| `cloudreaper-alerts_notifier_lambda_<version>_linux_<arch>.zip` | Notifier Lambda zip — drop-in for `var.lambda_zip` in Terraform |
| `cloudreaper-alerts_reactor_lambda_<version>_linux_<arch>.zip` | Reactor Lambda zip — drop-in for `var.reactor_zip` in Terraform |
| `checksums.txt` | SHA256 checksums for all artifacts |

## Running locally

Requires Docker, Terraform, and Go.

```bash
# Optional: real Slack delivery (otherwise placeholders are used)
export SLACK_BOT_TOKEN=xoxb-...
export SLACK_CHANNEL_ID=C0123ABCD

# Full demo: Floci → terraform apply → detect → (if orphans) send alert
bash scripts/run_local.sh

# Provision + detect, but don't actually POST the alert:
bash scripts/run_local.sh --dry-run
```

The script: starts Floci, builds the notifier Lambda zip, `terraform apply`s the scan
targets + pipeline, runs the detector (writes `build/report.{json,md}`, exits 1 on orphans), and
on orphans delivers the summary into SQS → Lambda → Slack.

## CI (Jenkins)

`Jenkinsfile` runs the same flow: provision → detect → on orphans, send a Slack alert in the
`post` block (and archives `build/report.*`). Set `SLACK_BOT_TOKEN` / `SLACK_CHANNEL_ID` as
Jenkins credentials for real delivery.

## Testing

```bash
go test ./...      # unit tests for detector, alert, slack, sender, notifier
```

The Go logic is unit-tested without AWS (fake EC2 client, httptest server, fake Slack poster).
Floci/Terraform are exercised by `scripts/run_local.sh`.
