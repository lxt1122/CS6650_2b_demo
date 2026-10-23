variable "cluster_name" {
  type        = string
  description = "ECS cluster name"
}

variable "service_name" {
  type        = string
  description = "ECS service name"
}

variable "min_capacity" {
  type        = number
  description = "Fewest tasks auto scaling keeps running"
}

variable "max_capacity" {
  type        = number
  description = "Most tasks auto scaling can run"
}

variable "cpu_target" {
  type        = number
  description = "Average CPU utilization (%) to keep the service near"
}

variable "scale_out_cooldown" {
  type        = number
  description = "Seconds to wait after scaling out before scaling out again"
}

variable "scale_in_cooldown" {
  type        = number
  description = "Seconds to wait after scaling in before scaling in again"
}
