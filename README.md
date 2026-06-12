<h1 align="center">CloudReaper Alerts</h1>
<img width="2377" height="1995" alt="bettershot_1781242034744" src="https://github.com/user-attachments/assets/c8b237fa-d84a-446a-b226-f5b48f918f04" />

<p align="center">
A self-contained <b>Go + Terraform</b> system that detects orphaned AWS resources and posts an
automatic Slack alert through an SQS-backed pipeline.
</p>


## Architecture

```
Jenkinsfile
  └─ terraform apply ──► Floci
        • scan targets: VPC, EC2, unattached EBS, unassociated EIP  (things to detect)
        • pipeline:     API Gateway POST /send-message ─► SQS (+DLQ) ─► Lambda ─► Slack
  └─ detector  ─ scans Floci, writes report.json, exits 1 if orphans found
  └─ sender    ─ on orphans, POSTs the orphan summary to API Gateway ─► … ─► Slack
```

## Layout

| Path | What |
|---|---|
| `cmd/detector` | CLI: scan Floci for orphans, write `report.json` |
| `cmd/sender`   | CLI: read `report.json`, POST orphan summary to the API Gateway |
| `cmd/notifier` | Lambda: SQS event → format → send to Slack |
| `internal/*`   | Testable logic for each of the above |
| `terraform/`   | Floci provider, scan targets, and the Slack pipeline |
| `scripts/run_local.sh` | One-shot local demo |
| `Jenkinsfile`  | CI orchestration |

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

> **HTTP front door vs. local path.** The Terraform provisions the slacked-style HTTP entry
> (`API Gateway POST /send-message` → SQS), which works on real AWS. Floci does **not** support
> API Gateway's AWS-service (→SQS) integration, so the local demo and Jenkins use
> `cmd/sender -queue-url …` to put the *identical* message straight on the queue. Everything
> downstream (SQS → Lambda → Slack) is the same either way. On real AWS, use
> `cmd/sender -endpoint <api-url>` instead (the `send_message_endpoint` output).

> Note: the notifier Lambda reaches Secrets Manager at `http://floci:4566` (Terraform sets
> `AWS_ENDPOINT_URL` on the function via the `lambda_internal_endpoint` var). Set it to `""`
> when deploying against real AWS.

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
