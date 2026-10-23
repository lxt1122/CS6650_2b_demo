variable "service_name" {
  type        = string
  description = "Base name for ALB resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC for the ALB and target group"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Subnets for the ALB (at least two availability zones)"
}

variable "container_port" {
  type        = number
  description = "Port the tasks listen on"
}
