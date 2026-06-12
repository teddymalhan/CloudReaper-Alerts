# orphan-watch

A self-contained **Go + Terraform** system that detects orphaned AWS resources and posts an
automatic Slack alert through an SQS-backed pipeline. It combines two ideas:

- the **orphan-detection** idea from `dummy-infra/` (now rewritten in Go), and
- the **Slack delivery pipeline** from `slacked/` (`API Gateway → SQS → Lambda → Slack`,
  reprovisioned here in Go + Terraform).

> `dummy-infra/` and `slacked/` are separate nested git repos used only as references — this
> project does not modify them.

## Architecture

```
Jenkinsfile
  └─ terraform apply ──► LocalStack
        • scan targets: VPC, EC2, unattached EBS, unassociated EIP  (things to detect)
        • pipeline:     API Gateway POST /send-message ─► SQS (+DLQ) ─► Lambda ─► Slack
  └─ detector  ─ scans LocalStack, writes report.json, exits 1 if orphans found
  └─ sender    ─ on orphans, POSTs the orphan summary to API Gateway ─► … ─► Slack
```

## Layout

| Path | What |
|---|---|
| `cmd/detector` | CLI: scan LocalStack for orphans, write `report.json` |
| `cmd/sender`   | CLI: read `report.json`, POST orphan summary to the API Gateway |
| `cmd/notifier` | Lambda: SQS event → format → send to Slack |
| `internal/*`   | Testable logic for each of the above |
| `terraform/`   | LocalStack provider, scan targets, and the Slack pipeline |
| `scripts/run_local.sh` | One-shot local demo |
| `Jenkinsfile`  | CI orchestration |

## Running locally

_Filled in as the build progresses (see `scripts/run_local.sh`)._

## Testing

```bash
go test ./...
```
