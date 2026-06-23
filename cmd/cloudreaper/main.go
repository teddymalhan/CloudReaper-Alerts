// Command cloudreaper installs and operates the AWS-native CloudReaper stack.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	cfn "github.com/teddymalhan/CloudReaper-Alerts/internal/cloudformation"
)

var version = "dev"

const (
	projectName = "cloudreaper"
	repoOwner   = "teddymalhan"
	repoName    = "CloudReaper-Alerts"
)

type artifact struct {
	name string
	path string
	key  string
}

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	var err error
	switch os.Args[1] {
	case "install":
		err = install(ctx, os.Args[2:])
	case "status":
		err = status(ctx, os.Args[2:])
	case "scan-now":
		err = invokeScan(ctx, os.Args[2:], false)
	case "test-alert":
		err = invokeScan(ctx, os.Args[2:], true)
	case "uninstall":
		err = uninstall(ctx, os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func install(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	region := fs.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region to deploy into")
	stackName := fs.String("stack-name", "cloudreaper-alerts", "CloudFormation stack name")
	slackWebhook := fs.String("slack-webhook", os.Getenv("SLACK_WEBHOOK_URL"), "Slack incoming webhook URL")
	schedule := fs.String("schedule", "rate(15 minutes)", "EventBridge schedule expression")
	realtime := fs.Bool("realtime", true, "enable CloudTrail/EventBridge near-real-time alerts")
	lambdaArch := fs.String("lambda-arch", defaultLambdaArch(), "Lambda architecture: x86_64 or arm64")
	release := fs.String("release", defaultRelease(), "GitHub release tag to download Lambda assets from")
	bucket := fs.String("artifact-bucket", "", "S3 bucket for Lambda zips; default cloudreaper-alerts-<account>-<region>")
	scannerZip := fs.String("scanner-zip", "", "local scanner Lambda zip instead of downloading release asset")
	notifierZip := fs.String("notifier-zip", "", "local notifier Lambda zip instead of downloading release asset")
	reactorZip := fs.String("reactor-zip", "", "local reactor Lambda zip instead of downloading release asset")
	initialScan := fs.Bool("initial-scan", true, "invoke scanner after deploy and send a clean test alert if no orphans are found")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *slackWebhook == "" {
		return fmt.Errorf("set --slack-webhook or SLACK_WEBHOOK_URL")
	}
	if *lambdaArch != "x86_64" && *lambdaArch != "arm64" {
		return fmt.Errorf("--lambda-arch must be x86_64 or arm64")
	}

	cfg, err := loadConfig(ctx, *region)
	if err != nil {
		return err
	}
	accountID, err := currentAccountID(ctx, cfg)
	if err != nil {
		return fmt.Errorf("resolve AWS account: %w", err)
	}
	if *bucket == "" {
		*bucket = fmt.Sprintf("cloudreaper-alerts-%s-%s", accountID, *region)
	}

	artifacts, err := prepareArtifacts(ctx, *release, *lambdaArch, *scannerZip, *notifierZip, *reactorZip)
	if err != nil {
		return err
	}
	if err := ensureBucket(ctx, cfg, *bucket, *region); err != nil {
		return err
	}
	if err := uploadArtifacts(ctx, cfg, *bucket, artifacts); err != nil {
		return err
	}

	params := map[string]string{
		"ProjectName":        projectName,
		"SlackWebhookUrl":    *slackWebhook,
		"ScanSchedule":       *schedule,
		"EnableRealtime":     fmt.Sprintf("%t", *realtime),
		"LambdaArchitecture": *lambdaArch,
		"ScannerS3Bucket":    *bucket,
		"ScannerS3Key":       artifacts["scanner"].key,
		"NotifierS3Bucket":   *bucket,
		"NotifierS3Key":      artifacts["notifier"].key,
		"ReactorS3Bucket":    *bucket,
		"ReactorS3Key":       artifacts["reactor"].key,
	}
	if err := deployStack(ctx, cfg, *stackName, params); err != nil {
		return err
	}
	outs, err := stackOutputs(ctx, cfg, *stackName)
	if err != nil {
		return err
	}
	fmt.Printf("CloudReaper installed in %s account %s.\n", *region, accountID)
	printOutputs(outs)
	if *initialScan {
		return invokeScanner(ctx, cfg, outs["ScannerFunctionName"], true)
	}
	return nil
}

func status(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	region := fs.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region")
	stackName := fs.String("stack-name", "cloudreaper-alerts", "CloudFormation stack name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(ctx, *region)
	if err != nil {
		return err
	}
	client := cloudformation.NewFromConfig(cfg)
	out, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: stackName})
	if err != nil {
		return err
	}
	if len(out.Stacks) == 0 {
		return fmt.Errorf("stack %s not found", *stackName)
	}
	fmt.Printf("Stack: %s\nStatus: %s\nRegion: %s\n", *stackName, out.Stacks[0].StackStatus, *region)
	printOutputs(outputsFromStack(out.Stacks[0]))
	return nil
}

func invokeScan(ctx context.Context, args []string, sendClean bool) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	region := fs.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region")
	stackName := fs.String("stack-name", "cloudreaper-alerts", "CloudFormation stack name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(ctx, *region)
	if err != nil {
		return err
	}
	outs, err := stackOutputs(ctx, cfg, *stackName)
	if err != nil {
		return err
	}
	return invokeScanner(ctx, cfg, outs["ScannerFunctionName"], sendClean)
}

func uninstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	region := fs.String("region", envOr("AWS_REGION", "us-east-1"), "AWS region")
	stackName := fs.String("stack-name", "cloudreaper-alerts", "CloudFormation stack name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig(ctx, *region)
	if err != nil {
		return err
	}
	client := cloudformation.NewFromConfig(cfg)
	if _, err := client.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: stackName}); err != nil {
		return err
	}
	fmt.Printf("Deleting stack %s...\n", *stackName)
	return cloudformation.NewStackDeleteCompleteWaiter(client).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: stackName}, 10*time.Minute)
}

func prepareArtifacts(ctx context.Context, release, arch, scannerPath, notifierPath, reactorPath string) (map[string]artifact, error) {
	assetArch := map[string]string{"x86_64": "amd64", "arm64": "arm64"}[arch]
	versionPart := strings.TrimPrefix(release, "v")
	assets := map[string]artifact{
		"scanner":  {name: fmt.Sprintf("%s_scanner_lambda_%s_linux_%s.zip", repoName, versionPart, assetArch), path: scannerPath},
		"notifier": {name: fmt.Sprintf("%s_notifier_lambda_%s_linux_%s.zip", repoName, versionPart, assetArch), path: notifierPath},
		"reactor":  {name: fmt.Sprintf("%s_reactor_lambda_%s_linux_%s.zip", repoName, versionPart, assetArch), path: reactorPath},
	}
	for id, a := range assets {
		if a.path == "" {
			path, err := downloadAsset(ctx, release, a.name)
			if err != nil {
				return nil, err
			}
			a.path = path
		}
		a.key = fmt.Sprintf("releases/%s/%s", release, filepath.Base(a.name))
		assets[id] = a
	}
	return assets, nil
}

func downloadAsset(ctx context.Context, release, name string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", repoOwner, repoName, release, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("download %s: %s", url, resp.Status)
	}
	path := filepath.Join(os.TempDir(), name)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", err
	}
	fmt.Printf("Downloaded %s\n", name)
	return path, nil
}

func ensureBucket(ctx context.Context, cfg aws.Config, bucket, region string) error {
	client := s3.NewFromConfig(cfg)
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	in := &s3.CreateBucketInput{Bucket: aws.String(bucket)}
	if region != "us-east-1" {
		in.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{LocationConstraint: s3types.BucketLocationConstraint(region)}
	}
	if _, err := client.CreateBucket(ctx, in); err != nil {
		return fmt.Errorf("create artifact bucket %s: %w", bucket, err)
	}
	return s3.NewBucketExistsWaiter(client).Wait(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)}, 2*time.Minute)
}

func uploadArtifacts(ctx context.Context, cfg aws.Config, bucket string, artifacts map[string]artifact) error {
	client := s3.NewFromConfig(cfg)
	for _, a := range artifacts {
		data, err := os.ReadFile(a.path)
		if err != nil {
			return err
		}
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(bucket),
			Key:         aws.String(a.key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String("application/zip"),
		})
		if err != nil {
			return fmt.Errorf("upload %s: %w", a.name, err)
		}
		fmt.Printf("Uploaded s3://%s/%s\n", bucket, a.key)
	}
	return nil
}

func deployStack(ctx context.Context, cfg aws.Config, stackName string, params map[string]string) error {
	client := cloudformation.NewFromConfig(cfg)
	_, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)})
	exists := err == nil
	parameters := make([]cfntypes.Parameter, 0, len(params))
	for k, v := range params {
		parameters = append(parameters, cfntypes.Parameter{ParameterKey: aws.String(k), ParameterValue: aws.String(v)})
	}
	caps := []cfntypes.Capability{cfntypes.CapabilityCapabilityNamedIam}
	if !exists {
		_, err = client.CreateStack(ctx, &cloudformation.CreateStackInput{
			StackName:    aws.String(stackName),
			TemplateBody: aws.String(cfn.Template),
			Parameters:   parameters,
			Capabilities: caps,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Creating CloudFormation stack %s...\n", stackName)
		return cloudformation.NewStackCreateCompleteWaiter(client).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}, 10*time.Minute)
	}
	_, err = client.UpdateStack(ctx, &cloudformation.UpdateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(cfn.Template),
		Parameters:   parameters,
		Capabilities: caps,
	})
	if err != nil {
		if strings.Contains(err.Error(), "No updates are to be performed") {
			fmt.Printf("Stack %s already up to date.\n", stackName)
			return nil
		}
		return err
	}
	fmt.Printf("Updating CloudFormation stack %s...\n", stackName)
	return cloudformation.NewStackUpdateCompleteWaiter(client).Wait(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)}, 10*time.Minute)
}

func stackOutputs(ctx context.Context, cfg aws.Config, stackName string) (map[string]string, error) {
	client := cloudformation.NewFromConfig(cfg)
	out, err := client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)})
	if err != nil {
		return nil, err
	}
	if len(out.Stacks) == 0 {
		return nil, fmt.Errorf("stack %s not found", stackName)
	}
	return outputsFromStack(out.Stacks[0]), nil
}

func outputsFromStack(stack cfntypes.Stack) map[string]string {
	outs := map[string]string{}
	for _, o := range stack.Outputs {
		outs[aws.ToString(o.OutputKey)] = aws.ToString(o.OutputValue)
	}
	return outs
}

func invokeScanner(ctx context.Context, cfg aws.Config, functionName string, sendClean bool) error {
	if functionName == "" {
		return errors.New("stack output ScannerFunctionName is empty")
	}
	payload, _ := json.Marshal(map[string]bool{"sendClean": sendClean})
	out, err := lambda.NewFromConfig(cfg).Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payload,
	})
	if err != nil {
		return err
	}
	if out.FunctionError != nil {
		return fmt.Errorf("scanner returned %s: %s", aws.ToString(out.FunctionError), string(out.Payload))
	}
	fmt.Printf("Scanner invoked: %s\n", strings.TrimSpace(string(out.Payload)))
	return nil
}

func loadConfig(ctx context.Context, region string) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, config.WithRegion(region))
}

func currentAccountID(ctx context.Context, cfg aws.Config) (string, error) {
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Account), nil
}

func printOutputs(outs map[string]string) {
	for _, key := range []string{"QueueUrl", "ScannerFunctionName", "NotifierFunctionName", "ReactorFunctionName", "SlackSecretName", "ScanScheduleRuleName"} {
		if val := outs[key]; val != "" {
			fmt.Printf("%s: %s\n", key, val)
		}
	}
}

func defaultRelease() string {
	if version != "" && version != "dev" {
		return "v" + strings.TrimPrefix(version, "v")
	}
	return "v0.2.0"
}

func defaultLambdaArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x86_64"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  cloudreaper install --slack-webhook URL [--region us-east-1]
  cloudreaper status [--region us-east-1]
  cloudreaper scan-now [--region us-east-1]
  cloudreaper test-alert [--region us-east-1]
  cloudreaper uninstall [--region us-east-1]
`)
}
