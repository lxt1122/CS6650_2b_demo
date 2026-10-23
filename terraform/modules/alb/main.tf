# Security group for the ALB: HTTP in from anywhere
resource "aws_security_group" "alb" {
  name        = "${var.service_name}-alb-sg"
  description = "Allow inbound HTTP on 80"
  vpc_id      = var.vpc_id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow HTTP traffic"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "Allow all outbound"
  }
}

# Application Load Balancer across the default VPC subnets
resource "aws_lb" "this" {
  name               = "${var.service_name}-alb"
  load_balancer_type = "application"
  subnets            = var.subnet_ids
  security_groups    = [aws_security_group.alb.id]
}

# Target group: Fargate tasks register by IP, and only tasks that pass
# the /health check receive traffic
resource "aws_lb_target_group" "this" {
  name        = "${var.service_name}-tg"
  target_type = "ip"
  protocol    = "HTTP"
  port        = var.container_port
  vpc_id      = var.vpc_id

  health_check {
    path              = "/health"
    interval          = 30
    healthy_threshold = 2
  }
}

# Listener: forward HTTP on port 80 to the target group
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.this.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.this.arn
  }
}
