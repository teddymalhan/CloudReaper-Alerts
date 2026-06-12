package detector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// fixedNow makes age/stopped-day math deterministic in tests.
var fixedNow = time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)

func withFixedNow(t *testing.T) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return fixedNow }
	t.Cleanup(func() { nowFunc = prev })
}

// fakeEC2 is a filter-aware in-memory EC2API.
type fakeEC2 struct {
	availableVolumes []types.Volume
	allVolumes       []types.Volume
	stoppedInstances []types.Instance
	allInstances     []types.Instance
	addresses        []types.Address
	err              error
}

func hasFilter(filters []types.Filter, name string) bool {
	for _, f := range filters {
		if aws.ToString(f.Name) == name {
			return true
		}
	}
	return false
}

func (f *fakeEC2) DescribeVolumes(_ context.Context, in *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	if hasFilter(in.Filters, "status") {
		return &ec2.DescribeVolumesOutput{Volumes: f.availableVolumes}, nil
	}
	return &ec2.DescribeVolumesOutput{Volumes: f.allVolumes}, nil
}

func (f *fakeEC2) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	insts := f.allInstances
	if hasFilter(in.Filters, "instance-state-name") {
		insts = f.stoppedInstances
	}
	return &ec2.DescribeInstancesOutput{Reservations: []types.Reservation{{Instances: insts}}}, nil
}

func (f *fakeEC2) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &ec2.DescribeAddressesOutput{Addresses: f.addresses}, nil
}

func tag(k, v string) types.Tag { return types.Tag{Key: aws.String(k), Value: aws.String(v)} }

func allRequiredTags() []types.Tag {
	return []types.Tag{tag("Project", "nimbuskart"), tag("Environment", "staging"), tag("Owner", "devops")}
}

func TestFindUnattachedEBS(t *testing.T) {
	withFixedNow(t)
	created := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC) // 3 days old
	f := &fakeEC2{availableVolumes: []types.Volume{{
		VolumeId:   aws.String("vol-123"),
		Size:       aws.Int32(8),
		CreateTime: &created,
		Tags:       allRequiredTags(),
	}}}

	got, err := FindUnattachedEBS(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	fnd := got[0]
	if fnd.ResourceID != "vol-123" || fnd.ResourceType != "ebs_volume" || fnd.Reason != "unattached" {
		t.Errorf("unexpected finding metadata: %+v", fnd)
	}
	if fnd.AgeDays != 3 {
		t.Errorf("age: want 3, got %d", fnd.AgeDays)
	}
	if fnd.EstimatedMonthlyCostUSD != 0.64 { // 8 * 0.08
		t.Errorf("cost: want 0.64, got %v", fnd.EstimatedMonthlyCostUSD)
	}
}

func TestFindLongStoppedInstances(t *testing.T) {
	withFixedNow(t)
	f := &fakeEC2{stoppedInstances: []types.Instance{
		{ // stopped ~41 days -> flagged
			InstanceId:            aws.String("i-old"),
			StateTransitionReason: aws.String("User initiated (2026-05-01 00:00:00 GMT)"),
			Tags:                  allRequiredTags(),
		},
		{ // stopped 2 days -> not flagged
			InstanceId:            aws.String("i-recent"),
			StateTransitionReason: aws.String("User initiated (2026-06-09 00:00:00 GMT)"),
			Tags:                  allRequiredTags(),
		},
	}}

	got, err := FindLongStoppedInstances(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	if got[0].ResourceID != "i-old" {
		t.Errorf("want i-old, got %s", got[0].ResourceID)
	}
	if got[0].EstimatedMonthlyCostUSD != 7.59 { // 0.0104 * 730
		t.Errorf("cost: want 7.59, got %v", got[0].EstimatedMonthlyCostUSD)
	}
}

func TestFindUnassociatedEIPs(t *testing.T) {
	withFixedNow(t)
	f := &fakeEC2{addresses: []types.Address{
		{AllocationId: aws.String("eipalloc-assoc"), AssociationId: aws.String("eipassoc-1")}, // skip
		{AllocationId: aws.String("eipalloc-orphan")},                                         // flag
	}}

	got, err := FindUnassociatedEIPs(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ResourceID != "eipalloc-orphan" {
		t.Fatalf("want only eipalloc-orphan, got %+v", got)
	}
	if got[0].EstimatedMonthlyCostUSD != 3.65 { // 0.005 * 730
		t.Errorf("cost: want 3.65, got %v", got[0].EstimatedMonthlyCostUSD)
	}
}

func TestFindMissingTags(t *testing.T) {
	withFixedNow(t)
	f := &fakeEC2{
		allInstances: []types.Instance{
			{InstanceId: aws.String("i-tagged"), Tags: allRequiredTags()},                                  // ok
			{InstanceId: aws.String("i-untagged"), Tags: []types.Tag{tag("Project", "x")}},                 // missing Environment,Owner
		},
		allVolumes: []types.Volume{
			{VolumeId: aws.String("vol-tagged"), Tags: allRequiredTags()}, // ok
		},
	}

	got, err := FindMissingTags(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ResourceID != "i-untagged" {
		t.Fatalf("want only i-untagged, got %+v", got)
	}
	if got[0].Reason != "missing_tags:Environment,Owner" {
		t.Errorf("reason: got %q", got[0].Reason)
	}
}

func TestScanAggregates(t *testing.T) {
	withFixedNow(t)
	created := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	f := &fakeEC2{
		availableVolumes: []types.Volume{{VolumeId: aws.String("vol-1"), Size: aws.Int32(8), CreateTime: &created, Tags: allRequiredTags()}},
		allVolumes:       []types.Volume{{VolumeId: aws.String("vol-1"), Size: aws.Int32(8), CreateTime: &created, Tags: allRequiredTags()}},
		addresses:        []types.Address{{AllocationId: aws.String("eipalloc-orphan")}},
	}

	rep, err := Scan(context.Background(), f, "us-east-1", "000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// unattached EBS (0.64) + unassociated EIP (3.65) = 2 findings.
	// (vol-1 has all required tags; no instances => no missing-tag findings.)
	if rep.Summary.TotalOrphans != 2 {
		t.Errorf("total orphans: want 2, got %d (%+v)", rep.Summary.TotalOrphans, rep.Findings)
	}
	if rep.Summary.EstimatedMonthlyWasteUSD != 4.29 { // 0.64 + 3.65
		t.Errorf("waste: want 4.29, got %v", rep.Summary.EstimatedMonthlyWasteUSD)
	}
	if rep.Region != "us-east-1" || rep.AccountID != "000000000000" {
		t.Errorf("report metadata wrong: %+v", rep)
	}
}

func TestScanPropagatesError(t *testing.T) {
	f := &fakeEC2{err: errors.New("boom")}
	if _, err := Scan(context.Background(), f, "us-east-1", "0"); err == nil {
		t.Fatal("expected error to propagate")
	}
}
