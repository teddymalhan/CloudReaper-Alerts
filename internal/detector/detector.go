package detector

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2API is the subset of the aws-sdk-go-v2 EC2 client the detectors use. Defining it as an
// interface lets tests inject a fake without touching AWS/LocalStack.
type EC2API interface {
	DescribeVolumes(ctx context.Context, in *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
	DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

// nowFunc is overridable in tests for deterministic age math.
var nowFunc = time.Now

func ageDays(t *time.Time) int {
	if t == nil {
		return 0
	}
	return int(nowFunc().Sub(*t).Hours() / 24)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// requiredTagSubset returns the required-tag keys with their values (nil when absent), matching
// dummy-infra's report shape.
func requiredTagSubset(tags []types.Tag) map[string]*string {
	have := map[string]string{}
	for _, t := range tags {
		have[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	out := map[string]*string{}
	for _, k := range RequiredTags {
		if v, ok := have[k]; ok {
			vv := v
			out[k] = &vv
		} else {
			out[k] = nil
		}
	}
	return out
}

func missingRequiredTags(tags []types.Tag) []string {
	have := map[string]bool{}
	for _, t := range tags {
		have[aws.ToString(t.Key)] = true
	}
	var missing []string
	for _, k := range RequiredTags {
		if !have[k] {
			missing = append(missing, k)
		}
	}
	return missing
}

// FindUnattachedEBS flags EBS volumes in the "available" state (not attached to an instance).
func FindUnattachedEBS(ctx context.Context, c EC2API) ([]Finding, error) {
	out, err := c.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
		Filters: []types.Filter{{Name: aws.String("status"), Values: []string{"available"}}},
	})
	if err != nil {
		return nil, fmt.Errorf("describe volumes: %w", err)
	}
	var findings []Finding
	for _, v := range out.Volumes {
		size := float64(aws.ToInt32(v.Size))
		findings = append(findings, Finding{
			ResourceID:              aws.ToString(v.VolumeId),
			ResourceType:            "ebs_volume",
			Reason:                  "unattached",
			AgeDays:                 ageDays(v.CreateTime),
			EstimatedMonthlyCostUSD: round2(size * EBSGp3PerGBMonth),
			Tags:                    requiredTagSubset(v.Tags),
			SuggestedAction:         "delete",
		})
	}
	return findings, nil
}

// CheckVolume runs a targeted orphan check on a single EBS volume id. It is the event-driven
// counterpart of FindUnattachedEBS: the reactor calls it in response to a DetachVolume CloudTrail
// event to confirm the just-detached volume is now "available" (unattached) before alerting.
// Returns no finding if the volume was re-attached or deleted in the meantime.
func CheckVolume(ctx context.Context, c EC2API, volumeID string) ([]Finding, error) {
	out, err := c.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{VolumeIds: []string{volumeID}})
	if err != nil {
		return nil, fmt.Errorf("describe volume %s: %w", volumeID, err)
	}
	var findings []Finding
	for _, v := range out.Volumes {
		if v.State != types.VolumeStateAvailable {
			continue
		}
		size := float64(aws.ToInt32(v.Size))
		findings = append(findings, Finding{
			ResourceID:              aws.ToString(v.VolumeId),
			ResourceType:            "ebs_volume",
			Reason:                  "unattached",
			AgeDays:                 ageDays(v.CreateTime),
			EstimatedMonthlyCostUSD: round2(size * EBSGp3PerGBMonth),
			Tags:                    requiredTagSubset(v.Tags),
			SuggestedAction:         "delete",
		})
	}
	return findings, nil
}

// FindLongStoppedInstances flags instances stopped for longer than StoppedDaysThreshold.
func FindLongStoppedInstances(ctx context.Context, c EC2API) ([]Finding, error) {
	out, err := c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		Filters: []types.Filter{{Name: aws.String("instance-state-name"), Values: []string{"stopped"}}},
	})
	if err != nil {
		return nil, fmt.Errorf("describe instances (stopped): %w", err)
	}
	var findings []Finding
	for _, r := range out.Reservations {
		for _, inst := range r.Instances {
			days := stoppedDays(aws.ToString(inst.StateTransitionReason))
			if days < StoppedDaysThreshold {
				continue
			}
			findings = append(findings, Finding{
				ResourceID:              aws.ToString(inst.InstanceId),
				ResourceType:            "ec2_instance",
				Reason:                  fmt.Sprintf("stopped_%d_days", days),
				AgeDays:                 days,
				EstimatedMonthlyCostUSD: round2(EC2T3MicroPerHour * HoursPerMonth),
				Tags:                    requiredTagSubset(inst.Tags),
				SuggestedAction:         "terminate",
			})
		}
	}
	return findings, nil
}

// stoppedDays parses "User initiated (2026-01-01 00:00:00 GMT)" and returns days since.
func stoppedDays(reason string) int {
	open := strings.Index(reason, "(")
	end := strings.Index(reason, " GMT)")
	if open < 0 || end < 0 || end <= open+1 {
		return 0
	}
	dateStr := reason[open+1 : end]
	t, err := time.Parse("2006-01-02 15:04:05", dateStr)
	if err != nil {
		return 0
	}
	t = t.UTC()
	return ageDays(&t)
}

// FindUnassociatedEIPs flags Elastic IPs with no association.
func FindUnassociatedEIPs(ctx context.Context, c EC2API) ([]Finding, error) {
	out, err := c.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe addresses: %w", err)
	}
	var findings []Finding
	for _, a := range out.Addresses {
		if aws.ToString(a.AssociationId) != "" {
			continue
		}
		id := aws.ToString(a.AllocationId)
		if id == "" {
			id = aws.ToString(a.PublicIp)
		}
		findings = append(findings, Finding{
			ResourceID:              id,
			ResourceType:            "elastic_ip",
			Reason:                  "unassociated",
			EstimatedMonthlyCostUSD: round2(EIPPerHour * HoursPerMonth),
			Tags:                    requiredTagSubset(a.Tags),
			SuggestedAction:         "release",
		})
	}
	return findings, nil
}

// FindMissingTags flags instances and volumes missing any required tag.
func FindMissingTags(ctx context.Context, c EC2API) ([]Finding, error) {
	var findings []Finding

	insts, err := c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe instances (tags): %w", err)
	}
	for _, r := range insts.Reservations {
		for _, inst := range r.Instances {
			if missing := missingRequiredTags(inst.Tags); len(missing) > 0 {
				findings = append(findings, Finding{
					ResourceID:      aws.ToString(inst.InstanceId),
					ResourceType:    "ec2_instance",
					Reason:          "missing_tags:" + strings.Join(missing, ","),
					Tags:            requiredTagSubset(inst.Tags),
					SuggestedAction: "tag",
				})
			}
		}
	}

	vols, err := c.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe volumes (tags): %w", err)
	}
	for _, v := range vols.Volumes {
		if missing := missingRequiredTags(v.Tags); len(missing) > 0 {
			findings = append(findings, Finding{
				ResourceID:      aws.ToString(v.VolumeId),
				ResourceType:    "ebs_volume",
				Reason:          "missing_tags:" + strings.Join(missing, ","),
				Tags:            requiredTagSubset(v.Tags),
				SuggestedAction: "tag",
			})
		}
	}
	return findings, nil
}

// Scan runs every detector and assembles the Report.
func Scan(ctx context.Context, c EC2API, region, accountID string) (Report, error) {
	var findings []Finding
	for _, fn := range []func(context.Context, EC2API) ([]Finding, error){
		FindUnattachedEBS, FindLongStoppedInstances, FindUnassociatedEIPs, FindMissingTags,
	} {
		f, err := fn(ctx, c)
		if err != nil {
			return Report{}, err
		}
		findings = append(findings, f...)
	}

	return NewReport(findings, region, accountID), nil
}

// NewReport rolls a set of findings up into a Report (summary totals + scan timestamp). Shared by
// the full Scan and the event-driven reactor so both emit an identical report/alert shape.
func NewReport(findings []Finding, region, accountID string) Report {
	var waste float64
	for _, f := range findings {
		waste += f.EstimatedMonthlyCostUSD
	}
	return Report{
		ScanTimestamp: nowFunc().UTC().Format("2006-01-02T15:04:05Z"),
		AccountID:     accountID,
		Region:        region,
		Summary: Summary{
			TotalOrphans:             len(findings),
			EstimatedMonthlyWasteUSD: round2(waste),
		},
		Findings: findings,
	}
}
