#!/usr/bin/env bash
# HW6 Part III: every 15 seconds, write the public IP of each running ECS task
# to targets/ecs-tasks.json. Prometheus (file_sd in prometheus-hw6.yml) reloads
# that file, so it scrapes every task directly as tasks scale out, scale in,
# or get replaced. Scraping through the ALB would hit a random task each time.
#
#   ./hw6-ecs-targets.sh        # Ctrl+C to stop
#
# Needs the AWS CLI (same credentials as terraform) and jq.
set -u

CLUSTER=${CLUSTER:-CS6650L2-cluster}
SERVICE=${SERVICE:-CS6650L2}
PORT=${PORT:-8080}
INTERVAL=${INTERVAL:-15}

cd "$(dirname "$0")"
mkdir -p targets
OUT=targets/ecs-tasks.json
last="start" # differs from any real result, so the first check always prints

while true; do
  if ! arns=$(aws ecs list-tasks --cluster "$CLUSTER" --service-name "$SERVICE" \
    --desired-status RUNNING --query 'taskArns[]' --output text 2>&1); then
    echo "$(date +%T) list-tasks failed: $arns"
    sleep "$INTERVAL"
    continue
  fi

  [ "$arns" = "None" ] && arns="" # text output of an empty list

  targets="[]"
  if [ -n "$arns" ]; then
    # $arns is a space-separated list, so it is left unquoted on purpose.
    tasks=$(aws ecs describe-tasks --cluster "$CLUSTER" --tasks $arns --output json)
    # Only RUNNING tasks: a task that is still starting has no public IP yet.
    enis=$(jq -r '.tasks[] | select(.lastStatus == "RUNNING")
      | .attachments[0].details[] | select(.name == "networkInterfaceId") | .value' <<<"$tasks")

    if [ -n "$enis" ]; then
      ips=$(aws ec2 describe-network-interfaces --network-interface-ids $enis --output json)
      # One file_sd entry per task, labelled with its task ID.
      targets=$(jq -n --argjson t "$tasks" --argjson n "$ips" --arg port "$PORT" '
        ($n.NetworkInterfaces | map({key: .NetworkInterfaceId, value: .Association.PublicIp})
          | from_entries) as $ip
        | [ $t.tasks[] | select(.lastStatus == "RUNNING")
            | (.attachments[0].details[] | select(.name == "networkInterfaceId") | .value) as $eni
            | select($ip[$eni] != null)
            | {targets: ["\($ip[$eni]):\($port)"], labels: {task: (.taskArn | split("/") | last)}} ]')
    fi
  fi

  # Write a temp file and rename it, so Prometheus never reads a half-written file.
  # The temp name does not end in .json, so file_sd ignores it.
  echo "$targets" >"$OUT.tmp" && mv "$OUT.tmp" "$OUT"

  now=$(jq -r '[.[].targets[0]] | join(" ")' <<<"$targets")
  if [ "$now" != "$last" ]; then
    echo "$(date +%T) $(jq length <<<"$targets") task(s): ${now:-none}"
    last=$now
  fi
  sleep "$INTERVAL"
done
