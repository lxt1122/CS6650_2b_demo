output "dns_name" {
  description = "Public DNS name of the ALB"
  value       = aws_lb.this.dns_name
}

output "arn_suffix" {
  description = "ALB ARN suffix, the LoadBalancer dimension of its CloudWatch metrics"
  value       = aws_lb.this.arn_suffix
}

output "target_group_arn" {
  description = "Target group the ECS service registers tasks in"
  value       = aws_lb_target_group.this.arn
}
