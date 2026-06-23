<h1 align="center">CloudReaper Alerts</h1>

<p align="center">
  <a href="https://github.com/teddymalhan/CloudReaper-Alerts/actions/workflows/go.yml"><img src="https://github.com/teddymalhan/CloudReaper-Alerts/actions/workflows/go.yml/badge.svg" alt="Build status"></a>
  <a href="https://github.com/teddymalhan/CloudReaper-Alerts/releases/latest"><img src="https://img.shields.io/github/v/release/teddymalhan/CloudReaper-Alerts" alt="Latest release"></a>
  <a href="https://github.com/teddymalhan/CloudReaper-Alerts/blob/main/LICENSE"><img src="https://img.shields.io/github/license/teddymalhan/CloudReaper-Alerts" alt="License"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/teddymalhan/CloudReaper-Alerts" alt="Go version"></a>
</p>

<img width="2377" height="1995" alt="bettershot_1781242034744" src="https://github.com/user-attachments/assets/c8b237fa-d84a-446a-b226-f5b48f918f04" />

<p align="center">
An AWS-native <b>Go + CloudFormation</b> system that deploys scheduled and near-real-time orphan
resource alerts into your AWS account, then posts findings to Slack through SQS-backed Lambdas.
</p>


## Architecture

<img width="2314" height="2091" alt="My First Board (3)" src="https://github.com/user-attachments/assets/e073ef5b-9284-4832-b46a-b8b6a290293e" />


## Install on AWS

Fast path: install the CLI, give it a Slack incoming webhook, and let it deploy the CloudFormation
stack into your account:

```bash
cloudreaper install \
  --region us-east-1 \
  --slack-webhook "$SLACK_WEBHOOK_URL"
```

The installer uploads the release Lambda zips to an account-local S3 bucket, deploys the
CloudFormation stack, runs an immediate scan, and sends a clean test alert when there are no
orphans. No EC2 host or daemon is required.

Daily commands:

```bash
cloudreaper status
cloudreaper scan-now
cloudreaper test-alert
cloudreaper uninstall
```

The AWS stack contains:

- EventBridge scheduled scanner Lambda (`rate(15 minutes)` by default)
- optional CloudTrail → EventBridge → reactor Lambda for near-real-time EC2 orphan events
- SQS main queue + DLQ
- notifier Lambda
- Secrets Manager Slack webhook secret

Terraform remains available under [`terraform/`](terraform/) as the legacy Floci/local target.

## Building

```bash
make package       # everything: CLIs → build/, Lambda zips → terraform/build/
make cli           # cloudreaper + detector + sender
make lambdas       # scanner.zip + notifier.zip + reactor.zip

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
| `CloudReaper-Alerts_cloudreaper_<version>_<os>_<arch>.tar.gz` | Installer/operator CLI — Linux / macOS / Windows, amd64 + arm64 |
| `CloudReaper-Alerts_detector_<version>_<os>_<arch>.tar.gz` | Standalone detector CLI — same platforms |
| `CloudReaper-Alerts_sender_<version>_<os>_<arch>.tar.gz` | Legacy sender CLI — same platforms |
| `CloudReaper-Alerts_scanner_lambda_<version>_linux_<arch>.zip` | Scheduled scanner Lambda zip — used by `cloudreaper install` |
| `CloudReaper-Alerts_notifier_lambda_<version>_linux_<arch>.zip` | Notifier Lambda zip — used by `cloudreaper install` and legacy Terraform |
| `CloudReaper-Alerts_reactor_lambda_<version>_linux_<arch>.zip` | Reactor Lambda zip — used by `cloudreaper install` and `var.reactor_zip` |
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
go test ./...      # unit tests for detector, alert, slack, sender, scanner, notifier, reactor
```

The Go logic is unit-tested without AWS (fake EC2/SQS clients, httptest server, fake Slack poster).
Floci/Terraform are exercised by `scripts/run_local.sh`; CloudFormation is the primary AWS install
target.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
development workflow, and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for
community standards. Security issues should be reported per
[SECURITY.md](SECURITY.md) rather than as a public issue.

## License

[MIT](LICENSE)
