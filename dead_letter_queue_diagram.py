from diagrams import Cluster, Diagram, Edge
from diagrams.aws.compute import ElasticKubernetesService, Fargate
from diagrams.aws.database import Dynamodb
from diagrams.aws.devtools import XRay
from diagrams.aws.integration import Eventbridge, SimpleQueueServiceSqs, StepFunctions
from diagrams.aws.management import Cloudformation, Cloudwatch, CloudwatchAlarm, CloudwatchLogs, SystemsManager
from diagrams.aws.network import APIGateway, Privatelink, VPC
from diagrams.aws.security import IdentityAndAccessManagementIam, KeyManagementService, SecretsManager
from diagrams.aws.storage import S3
from diagrams.k8s.compute import Deployment, Pod
from diagrams.k8s.network import Ingress
from diagrams.k8s.rbac import ServiceAccount

GRAPH_ATTR = {
    "fontsize": "24",
    "pad": "0.6",
    "ranksep": "1.0",
    "nodesep": "0.7",
    "splines": "ortho",
}

NODE_ATTR = {"fontsize": "12"}
EDGE_ATTR = {"fontsize": "10", "color": "#475569", "arrowsize": "0.8"}
FAIL = "#dc2626"
REPLAY = "#2563eb"
SECURITY = "#7c3aed"
OBSERVE = "#059669"

with Diagram(
    "AWS SQS Dead Letter Queue on EKS",
    filename="dead_letter_queue_architecture",
    outformat=["png", "svg"],
    show=False,
    direction="LR",
    graph_attr=GRAPH_ATTR,
    node_attr=NODE_ATTR,
    edge_attr=EDGE_ATTR,
):
    with Cluster("Decision 1: SQS owns durability and retry state"):
        api = APIGateway("API Gateway\naccepts work")
        events = Eventbridge("EventBridge\nnormalizes events")
        source_queue = SimpleQueueServiceSqs("orders-work.fifo\nSQS source queue\nvisibility timeout: 6x pod timeout\nredrive after 5 receives")
        dlq = SimpleQueueServiceSqs("orders-work-dlq.fifo\nSQS dead-letter queue\nretention: 14 days\nKMS encrypted")
        api >> events >> source_queue
        source_queue >> Edge(label="maxReceiveCount = 5", color=FAIL) >> dlq

    with Cluster("Decision 2: EKS workers are stateless; SQS is the buffer"):
        vpc = VPC("Private VPC")
        endpoint = Privatelink("VPC endpoint\ncom.amazonaws.sqs")
        eks = ElasticKubernetesService("EKS cluster")
        ingress = Ingress("internal ingress")
        app_sa = ServiceAccount("order-worker SA\nIRSA enabled")
        worker = Deployment("order-worker Deployment\nKEDA scales on SQS depth")
        pods = Pod("worker pods\nlong-poll SQS\nDeleteMessage only after commit")
        vpc >> endpoint >> source_queue
        eks >> ingress >> worker >> pods
        app_sa >> Edge(label="assumed by pods") >> pods
        pods >> Edge(label="ReceiveMessage / DeleteMessage") >> endpoint

    with Cluster("Decision 3: Idempotent writes before ACK"):
        idempotency = Dynamodb("DynamoDB idempotency table\nmessageId + business key")
        business_store = Dynamodb("DynamoDB orders table\nconditional writes")
        pods >> Edge(label="check / reserve key") >> idempotency
        pods >> Edge(label="commit side effect") >> business_store
        business_store >> Edge(label="commit ok") >> pods
        idempotency >> Edge(label="duplicate = ack safely") >> pods

    with Cluster("Decision 4: DLQ recovery is explicit, not automatic"):
        triage_sa = ServiceAccount("dlq-triage SA\nread DLQ, replay source")
        triage = Deployment("dlq-triage Deployment\nmanual scale from 0")
        triage_pods = Pod("triage pods\ninspect poison messages")
        remediation = StepFunctions("remediation workflow\napprove replay or quarantine")
        replay_queue = SimpleQueueServiceSqs("orders-replay.fifo\nrate-limited replay queue")
        quarantine = S3("S3 quarantine bucket\nraw body + headers + error")
        triage_sa >> triage >> triage_pods
        dlq >> Edge(label="ReceiveMessage", color=REPLAY) >> triage_pods
        triage_pods >> remediation
        remediation >> Edge(label="safe to retry", color=REPLAY) >> replay_queue >> source_queue
        remediation >> Edge(label="bad payload", color=FAIL) >> quarantine

    with Cluster("Decision 5: Access is narrow and encrypted"):
        kms = KeyManagementService("KMS CMK\nSQS + S3 encryption")
        worker_role = IdentityAndAccessManagementIam("IAM role: worker\nsource queue read/delete only")
        triage_role = IdentityAndAccessManagementIam("IAM role: triage\nDLQ read/delete + replay send")
        secrets = SecretsManager("Secrets Manager\nDB credentials/API keys")
        kms >> Edge(label="encrypts", color=SECURITY) >> [source_queue, dlq, replay_queue, quarantine]
        worker_role >> Edge(label="IRSA", color=SECURITY) >> app_sa
        triage_role >> Edge(label="IRSA", color=SECURITY) >> triage_sa
        secrets >> Edge(label="mounted at runtime", color=SECURITY) >> pods

    with Cluster("Decision 6: Operators page on impact, not noise"):
        logs = CloudwatchLogs("CloudWatch Logs\napp error + message metadata")
        metrics = Cloudwatch("CloudWatch metrics\nDLQ visible, oldest age")
        alarm = CloudwatchAlarm("Alarm\nDLQ depth > 0 for 5m\nor oldest age > 30m")
        traces = XRay("X-Ray\ntrace message lifecycle")
        runbook = SystemsManager("SSM Automation\nscale triage + start replay")
        pods >> Edge(color=OBSERVE) >> logs
        triage_pods >> Edge(color=OBSERVE) >> logs
        source_queue >> Edge(color=OBSERVE) >> metrics
        dlq >> Edge(color=OBSERVE) >> metrics >> alarm >> runbook
        pods >> Edge(color=OBSERVE) >> traces
        triage_pods >> Edge(color=OBSERVE) >> traces

    with Cluster("Decision 7: Infrastructure is reproducible"):
        iac = Cloudformation("CloudFormation / CDK\nqueues, policies, alarms, IRSA")
        fargate = Fargate("EKS Fargate profile\noptional for triage pods")
        iac >> [source_queue, dlq, replay_queue, worker_role, triage_role, alarm]
        fargate >> triage_pods
