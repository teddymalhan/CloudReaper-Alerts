package reactor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/teddymalhan/CloudReaper-Alerts/internal/alert"
)

// fakeEC2 is a minimal EC2API: it echoes back whatever the test stages, regardless of filters.
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

// fakeSQS captures the message bodies the handler enqueues.
type fakeSQS struct{ bodies []string }

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.bodies = append(f.bodies, aws.ToString(in.MessageBody))
	return &sqs.SendMessageOutput{}, nil
}

func detachVolumeDetail(t *testing.T, volumeID string) json.RawMessage {
	t.Helper()
	d := Detail{EventName: "DetachVolume", RequestParameters: json.RawMessage(`{"volumeId":"` + volumeID + `"}`)}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal detail: %v", err)
	}
	return b
}

func TestCheckRoutesDetachVolume(t *testing.T) {
	f := &fakeEC2{volumes: []types.Volume{{
		VolumeId: aws.String("vol-abc"),
		Size:     aws.Int32(10),
		State:    types.VolumeStateAvailable,
	}}}
	d := Detail{EventName: "DetachVolume", RequestParameters: json.RawMessage(`{"volumeId":"vol-abc"}`)}

	got, err := Check(context.Background(), f, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ResourceID != "vol-abc" {
		t.Fatalf("want one finding for vol-abc, got %+v", got)
	}
}

func TestCheckIgnoresUnhandledEvent(t *testing.T) {
	got, err := Check(context.Background(), &fakeEC2{}, Detail{EventName: "ReleaseAddress"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReleaseAddress should be ignored, got %+v", got)
	}
}

func TestHandlerEnqueuesConfirmedOrphan(t *testing.T) {
	ec2c := &fakeEC2{volumes: []types.Volume{{
		VolumeId: aws.String("vol-abc"),
		Size:     aws.Int32(10),
		State:    types.VolumeStateAvailable,
	}}}
	sqsc := &fakeSQS{}
	h := Handler{EC2: ec2c, SQS: sqsc, QueueURL: "http://queue/main"}

	ev := events.CloudWatchEvent{Region: "us-east-1", AccountID: "000000000000", Detail: detachVolumeDetail(t, "vol-abc")}
	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(sqsc.bodies) != 1 {
		t.Fatalf("want one enqueued alert, got %d", len(sqsc.bodies))
	}

	var a alert.OrphanAlert
	if err := json.Unmarshal([]byte(sqsc.bodies[0]), &a); err != nil {
		t.Fatalf("enqueued body is not an OrphanAlert: %v", err)
	}
	if a.Source != "orphan-watch:reactor" {
		t.Errorf("source: want orphan-watch:reactor, got %q", a.Source)
	}
	if a.TotalOrphans != 1 || len(a.Findings) != 1 || a.Findings[0].ResourceID != "vol-abc" {
		t.Errorf("unexpected alert payload: %+v", a)
	}
}

func TestHandlerSkipsWhenNotOrphaned(t *testing.T) {
	// Volume was re-attached before the event was processed: in-use, so no alert.
	ec2c := &fakeEC2{volumes: []types.Volume{{VolumeId: aws.String("vol-abc"), State: types.VolumeStateInUse}}}
	sqsc := &fakeSQS{}
	h := Handler{EC2: ec2c, SQS: sqsc, QueueURL: "http://queue/main"}

	ev := events.CloudWatchEvent{Region: "us-east-1", Detail: detachVolumeDetail(t, "vol-abc")}
	if err := h.Handle(context.Background(), ev); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if len(sqsc.bodies) != 0 {
		t.Fatalf("re-attached volume should produce no alert, got %d", len(sqsc.bodies))
	}
}
