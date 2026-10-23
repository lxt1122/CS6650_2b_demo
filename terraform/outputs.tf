output "ecs_cluster_name" {
  description = "Name of the created ECS cluster"
  value       = module.ecs.cluster_name
}

output "ecs_service_name" {
  description = "Name of the running ECS service"
  value       = module.ecs.service_name
}

output "alb_dns_name" {
  description = "Public DNS name of the ALB (load test host: http://<this>)"
  value       = module.alb.dns_name
}