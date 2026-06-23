package scanner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
)

type fakeEC2 struct {
	volumes   []types.Volume
	addresses []types.Address
}

func (f *fakeEC2) DescribeVolumes(context.Context, *ec2.DescribeVolumesInput, ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{Volumes: f.volumes}, nil
}
func (f *fakeEC2) DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (f *fakeEC2) DescribeAddresses(context.Context, *ec2.DescribeAddressesInput, ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{Addresses: f.addresses}, nil
}

type fakeSQS struct{ bodies []string }

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.bodies = append(f.bodies, aws.ToString(in.MessageBody))
	return &sqs.SendMessageOutput{}, nil
}

func TestHandleQueuesFindings(t *testing.T) {
	sqsClient := &fakeSQS{}
	h := Handler{
		EC2:       &fakeEC2{addresses: []types.Address{{AllocationId: aws.String("eipalloc-orphan")}}},
		SQS:       sqsClient,
		QueueURL:  "https://sqs.us-east-1.amazonaws.com/123/cloudreaper",
		Region:    "us-east-1",
		AccountID: "123",
	}

	result, err := h.Handle(context.Background(), Event{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AlertQueued || result.TotalOrphans != 1 {
		t.Fatalf("expected queued finding result, got %+v", result)
	}
	if len(sqsClient.bodies) != 1 {
		t.Fatalf("expected one queued alert, got %d", len(sqsClient.bodies))
	}
	var queued alert.OrphanAlert
	if err := json.Unmarshal([]byte(sqsClient.bodies[0]), &queued); err != nil {
		t.Fatalf("queued body is not alert JSON: %v", err)
	}
	if queued.AccountID != "123" || queued.Region != "us-east-1" || queued.TotalOrphans != 1 {
		t.Fatalf("unexpected queued alert: %+v", queued)
	}
}

func TestHandleSuppressesCleanScheduledScan(t *testing.T) {
	sqsClient := &fakeSQS{}
	h := Handler{EC2: &fakeEC2{}, SQS: sqsClient, QueueURL: "queue", Region: "us-east-1", AccountID: "123"}

	result, err := h.Handle(context.Background(), Event{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AlertQueued || result.TotalOrphans != 0 || len(sqsClient.bodies) != 0 {
		t.Fatalf("clean scheduled scan should be silent, result=%+v queued=%d", result, len(sqsClient.bodies))
	}
}

func TestHandleCanSendCleanTestAlert(t *testing.T) {
	sqsClient := &fakeSQS{}
	h := Handler{EC2: &fakeEC2{}, SQS: sqsClient, QueueURL: "queue", Region: "us-east-1", AccountID: "123"}

	result, err := h.Handle(context.Background(), Event{SendClean: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AlertQueued || result.TotalOrphans != 0 || len(sqsClient.bodies) != 1 {
		t.Fatalf("clean test alert should be queued, result=%+v queued=%d", result, len(sqsClient.bodies))
	}
}
