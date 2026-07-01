terraform {
  required_version = ">= 1.9"
  required_providers {
    # Pin below 5.82 to avoid ValidateStateMachineDefinition call unsupported by LocalStack Community.
    # For real AWS, remove the upper bound.
    aws = { source = "hashicorp/aws", version = ">= 5.0, < 5.50" }
  }
}

provider "aws" {
  region                      = var.region
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true
  endpoints {
    iam           = var.endpoint
    logs          = var.endpoint
    sts           = var.endpoint
    codepipeline  = var.endpoint
    codebuild     = var.endpoint
    codedeploy    = var.endpoint
    events        = var.endpoint
    stepfunctions = var.endpoint
  }
}
