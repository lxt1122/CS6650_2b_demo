# CloudWatch dashboard with the HW6 Part III key metrics, all at 1-minute resolution:
# CPU per task, ALB request count and target response time, desired vs running tasks.
resource "aws_cloudwatch_dashboard" "this" {
  dashboard_name = "${var.service_name}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "ECS CPU utilization per task (%)"
          region = var.region
          view   = "timeSeries"
          period = 60
          # SEARCH finds every task of the service, so tasks added by
          # auto scaling get their own line without editing the dashboard.
          metrics = [[{
            id         = "cpu"
            expression = "SEARCH('{ECS/ContainerInsights,ClusterName,ServiceName,TaskId} MetricName=\"TaskCpuUtilization\" ClusterName=\"${var.cluster_name}\" ServiceName=\"${var.service_name}\"', 'Average', 60)"
          }]]
          yAxis = { left = { min = 0, max = 100 } }
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title  = "Desired vs running tasks"
          region = var.region
          view   = "timeSeries"
          stat   = "Average"
          period = 60
          metrics = [
            ["ECS/ContainerInsights", "DesiredTaskCount", "ServiceName", var.service_name, "ClusterName", var.cluster_name],
            ["ECS/ContainerInsights", "RunningTaskCount", "ServiceName", var.service_name, "ClusterName", var.cluster_name],
          ]
          yAxis = { left = { min = 0 } }
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "ALB request count (per minute)"
          region  = var.region
          view    = "timeSeries"
          stat    = "Sum"
          period  = 60
          metrics = [["AWS/ApplicationELB", "RequestCount", "LoadBalancer", var.alb_arn_suffix]]
          yAxis   = { left = { min = 0 } }
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "ALB target response time (seconds)"
          region  = var.region
          view    = "timeSeries"
          stat    = "Average"
          period  = 60
          metrics = [["AWS/ApplicationELB", "TargetResponseTime", "LoadBalancer", var.alb_arn_suffix]]
          yAxis   = { left = { min = 0 } }
        }
      },
    ]
  })
}
