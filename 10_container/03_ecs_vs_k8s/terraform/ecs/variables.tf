variable "endpoint" {
  type        = string
  description = "AWS endpoint (LocalStack のときは http://localhost:4566)"
  default     = "http://localhost:4566"
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "ecr_repo" {
  type        = string
  description = "Container image repository URI (e.g. localhost:5000/demo-api)"
  default     = "localhost:5000/demo-api"
}
