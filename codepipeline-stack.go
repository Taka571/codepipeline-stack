package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk"
	"github.com/aws/aws-cdk-go/awscdk/awscodebuild"
	"github.com/aws/aws-cdk-go/awscdk/awsiam"
	"github.com/aws/constructs-go/constructs/v3"
	"github.com/aws/jsii-runtime-go"

	"github.com/aws/aws-cdk-go/awscdk/awscodepipeline"
	"github.com/aws/aws-cdk-go/awscdk/awscodepipelineactions"
)

type CodepipelineStackProps struct {
	awscdk.StackProps
}

type Actions struct {
	sourceAction            awscodepipelineactions.GitHubSourceAction
	approvalAction          awscodepipelineactions.ManualApprovalAction
	applicationDeployAction awscodepipelineactions.CodeBuildAction
}

var sprops awscdk.StackProps
var appOutput awscodepipeline.Artifact

func NewCodepipelineStack(scope constructs.Construct, id string, props *CodepipelineStackProps) awscdk.Stack {
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)
	appOutput = awscodepipeline.NewArtifact(jsii.String("SampleArtifact"))

	sourceAction := createGithubSourceAction()
	approvalAction := awscodepipelineactions.NewManualApprovalAction(&awscodepipelineactions.ManualApprovalActionProps{
		ActionName:         jsii.String("SampleDeployApprovalAction"),
		RunOrder:           jsii.Number(2),
		ExternalEntityLink: sourceAction.Variables().CommitUrl,
	})
	pipelineProject := createPipelineProject(stack)
	applicationDeployAction := createCodeBuildAction(pipelineProject)

	createPipeline(stack, Actions{
		sourceAction:            sourceAction,
		approvalAction:          approvalAction,
		applicationDeployAction: applicationDeployAction,
	})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	NewCodepipelineStack(app, "CodepipelineStack", &CodepipelineStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {
	return &awscdk.Environment{
		Account: jsii.String(os.Getenv("CDK_DEFAULT_ACCOUNT")),
		Region:  jsii.String(os.Getenv("CDK_DEFAULT_REGION")),
	}
}

// Registration your github token with AWS Systems Manager is required before deploy
func createGithubSourceAction() awscodepipelineactions.GitHubSourceAction {
	// replace "GithubToken" to your secret name
	gitHubToken := awscdk.SecretValue_SecretsManager(jsii.String("GithubToken"), &awscdk.SecretsManagerSecretOptions{JsonField: jsii.String("GithubToken")})
	sourceActionProps := awscodepipelineactions.GitHubSourceActionProps{
		ActionName: jsii.String("GitHubSourceAction"),
		Owner:      jsii.String("Taka571"), // your name
		OauthToken: gitHubToken,
		Repo:       jsii.String("codepipeline-sample"), // your repository
		Branch:     jsii.String("main"),
		Output:     appOutput,
		RunOrder:   jsii.Number(1),
		Trigger:    awscodepipelineactions.GitHubTrigger_WEBHOOK,
	}
	sourceAction := awscodepipelineactions.NewGitHubSourceAction(&sourceActionProps)
	return sourceAction
}

func createPipelineProject(scope constructs.Construct) awscodebuild.PipelineProject {
	policyStatement := awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions: &[]*string{
			jsii.String("ecr:BatchCheckLayerAvailability"),
			jsii.String("ecr:GetAuthorizationToken"),
			jsii.String("ecr:PutImage"),
			jsii.String("ecr:InitiateLayerUpload"),
			jsii.String("ecr:UploadLayerPart"),
			jsii.String("ecr:CompleteLayerUpload"),
		},
		Effect:    awsiam.Effect_ALLOW,
		Resources: &[]*string{jsii.String("*")},
	})

	policyDocument := awsiam.NewPolicyDocument(&awsiam.PolicyDocumentProps{
		Statements: &[]awsiam.PolicyStatement{policyStatement},
	})

	inlinePolicies := make(map[string]awsiam.PolicyDocument)
	inlinePolicies["ecrAccessPolicy"] = policyDocument

	servicePrincipal := awsiam.NewServicePrincipal(jsii.String("codebuild.amazonaws.com"), &awsiam.ServicePrincipalOpts{})

	deployRole := awsiam.NewRole(scope, jsii.String("SampleCodeBuildDeployRole"), &awsiam.RoleProps{
		InlinePolicies: &inlinePolicies,
		AssumedBy:      servicePrincipal,
	})

	environmentVariables := make(map[string]*awscodebuild.BuildEnvironmentVariable)
	environmentVariables["AWS_REGION"] = &awscodebuild.BuildEnvironmentVariable{
		Value: sprops.Env.Region,
		Type:  awscodebuild.BuildEnvironmentVariableType_PLAINTEXT,
	}
	environmentVariables["AWS_ACCOUNT_ID"] = &awscodebuild.BuildEnvironmentVariable{
		Value: sprops.Env.Account,
		Type:  awscodebuild.BuildEnvironmentVariableType_PLAINTEXT,
	}

	project := awscodebuild.NewPipelineProject(scope, jsii.String("SampleProject"), &awscodebuild.PipelineProjectProps{
		Cache:                               awscodebuild.Cache_Local(awscodebuild.LocalCacheMode_SOURCE, awscodebuild.LocalCacheMode_CUSTOM),
		CheckSecretsInPlainTextEnvVariables: jsii.Bool(true),
		Environment: &awscodebuild.BuildEnvironment{
			BuildImage:           awscodebuild.LinuxBuildImage_STANDARD_5_0(),
			ComputeType:          awscodebuild.ComputeType_SMALL,
			EnvironmentVariables: &environmentVariables,
			Privileged:           jsii.Bool(true),
		},
		BuildSpec:     awscodebuild.BuildSpec_FromSourceFilename(jsii.String("buildspec.yml")),
		Role:          deployRole,
		ProjectName:   jsii.String("SampleApplicationDeployProject"),
		QueuedTimeout: awscdk.Duration_Minutes(jsii.Number(15)),
		Timeout:       awscdk.Duration_Minutes(jsii.Number(5)),
	})

	return project
}

func createCodeBuildAction(project awscodebuild.PipelineProject) awscodepipelineactions.CodeBuildAction {
	applicationDeployAction := awscodepipelineactions.NewCodeBuildAction(&awscodepipelineactions.CodeBuildActionProps{
		ActionName: jsii.String("SampleApplicationDeployAction"),
		Project:    project,
		Input:      appOutput,
		RunOrder:   jsii.Number(3),
	})

	return applicationDeployAction
}

func createPipeline(scope constructs.Construct, actions Actions) {
	pipeline := awscodepipeline.NewPipeline(scope, jsii.String("SampleApplicationDeployPipeline"), &awscodepipeline.PipelineProps{
		PipelineName: jsii.String("SampleApplicationDeployPipeline"),
	})

	pipeline.AddStage(&awscodepipeline.StageOptions{
		StageName: jsii.String("SampleGitHubSourceActionStage"),
		Actions:   &[]awscodepipeline.IAction{actions.sourceAction},
	})

	pipeline.AddStage(&awscodepipeline.StageOptions{
		StageName: jsii.String("SampleApplicationDeployStage"),
		Actions:   &[]awscodepipeline.IAction{actions.approvalAction, actions.applicationDeployAction},
	})
}
