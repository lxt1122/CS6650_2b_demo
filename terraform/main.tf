# Wire together seven focused modules: network, ecr, logging, alb, ecs,
# autoscaling, dashboard.

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "alb" {
  source         = "./modules/alb"
  service_name   = var.service_name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.subnet_ids
  container_port = var.container_port
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# Reuse an existing IAM role for ECS tasks
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  cpu                = var.cpu
  memory             = var.memory
  target_group_arn   = module.alb.target_group_arn
  region             = var.aws_region

  # The target group must be attached to the ALB (via the listener)
  # before the service can register tasks in it.
  depends_on = [module.alb]
}

module "autoscaling" {
  source             = "./modules/autoscaling"
  cluster_name       = module.ecs.cluster_name
  service_name       = module.ecs.service_name
  min_capacity       = var.min_capacity
  max_capacity       = var.max_capacity
  cpu_target         = var.cpu_target
  scale_out_cooldown = var.scale_out_cooldown
  scale_in_cooldown  = var.scale_in_cooldown
}

module "dashboard" {
  source         = "./modules/dashboard"
  service_name   = module.ecs.service_name
  cluster_name   = module.ecs.cluster_name
  alb_arn_suffix = module.alb.arn_suffix
  region         = var.aws_region
}


// Build & push the Go app image into ECR
resource "docker_image" "app" {
  # Use the URL from the ecr module, and tag it "latest"
  name = "${module.ecr.repository_url}:latest"

  build {
    # relative path from terraform/ → src/
    context = "../src"
    # Dockerfile defaults to "Dockerfile" in that context
    platform = "linux/amd64"
  }
}

resource "docker_registry_image" "app" {
  # this will push :latest → ECR
  name = docker_image.app.name
}
