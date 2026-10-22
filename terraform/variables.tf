# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "CS6650L2"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

# Fargate task size. 256 CPU units = 0.25 vCPU.
# Fargate needs at least 1024 MiB of memory when cpu is 512.
variable "cpu" {
  type    = string
  default = "256"
}

variable "memory" {
  type    = string
  default = "512"
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}
