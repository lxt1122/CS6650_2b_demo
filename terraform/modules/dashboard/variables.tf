variable "service_name" {
  type        = string
  description = "ECS service name, also the dashboard name prefix"
}

variable "cluster_name" {
  type        = string
  description = "ECS cluster name"
}

variable "alb_arn_suffix" {
  type        = string
  description = "ALB ARN suffix (LoadBalancer dimension)"
}

variable "region" {
  type        = string
  description = "Region the metrics are in"
}
