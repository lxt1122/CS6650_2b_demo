# CS6650 HW5 – Product API

A Gin server implementing the Product endpoints of `api.yaml`, deployed to AWS ECS (Fargate) with Terraform, load tested with Locust.

## Repo layout

| Path | What it is |
|---|---|
| `src/product-service.go` | Product API server (Gin, in-memory map guarded by an RWMutex, IDs 1–1000 seeded at startup) |
| `src/Dockerfile` | Container image for the server |
| `terraform/` | Infrastructure: `provider.tf`, `variables.tf`, `main.tf`, `outputs.tf`, and `modules/` (network, ecr, logging, ecs) |
| `postman/hw5-product-api.postman_collection.json` | Postman collection that triggers every response code |
| `locust/` | Locust load tests (`locustfile.py` = HttpUser, `locustfile_fast.py` = FastHttpUser) |
| `screenshots/` | Test results |

## Deploy to AWS

**Prerequisites:** AWS CLI, Terraform, and Docker (running).

1. Copy temporary credentials from AWS Learner Lab, then:
   ```
   aws configure
   aws configure set aws_session_token <YOUR-TEMP-SESSION-TOKEN>
   ```
2. Build the image, push it to ECR, and start the ECS service:
   ```
   cd terraform
   terraform init
   terraform apply -auto-approve
   ```
   The image is built for `linux/amd64` (Fargate's default CPU), so on an Apple Silicon Mac the build runs under emulation and takes about a minute.
3. Get the task's public IP (run from `terraform/`):
   ```
   aws ec2 describe-network-interfaces \
   --network-interface-ids $(
       aws ecs describe-tasks \
       --cluster $(terraform output -raw ecs_cluster_name) \
       --tasks $(
           aws ecs list-tasks \
           --cluster $(terraform output -raw ecs_cluster_name) \
           --service-name $(terraform output -raw ecs_service_name) \
           --query 'taskArns[0]' --output text
       ) \
       --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
       --output text
   ) \
   --query 'NetworkInterfaces[0].Association.PublicIp' \
   --output text
   ```
4. Send a request:
   ```
   curl http://<PUBLIC-IP>:8080/products/1
   ```
5. Clean up:
   ```
   terraform destroy -auto-approve
   ```

To run locally instead: `cd src && go run product-service.go`, or `docker build -t product-service src && docker run --rm -p 8080:8080 product-service`.

## API and response codes

| Endpoint | Codes |
|---|---|
| `GET /products/{productId}` | 200, 404, 500 |
| `POST /products/{productId}/details` | 204, 400, 404, 500 |

Products 1–1000 exist at startup; POST updates an existing product and returns 404 for any other ID. Sending the header `X-Simulate-Error: true` returns 500 on demand.

**Run every case:** import `postman/hw5-product-api.postman_collection.json` into Postman, set the collection variable `baseUrl` (default `http://localhost:8080`), then **Run collection**.

| Request | Code |
|---|---|
| `GET /products/1` | 200 |
| `GET /products/99999` or `/products/abc` | 404 |
| `GET /products/1` with `X-Simulate-Error: true` | 500 |
| `POST /products/2/details` with a valid body | 204 |
| `POST` with a missing field, `weight: -1`, `weight: 12.5`, body/path ID mismatch, malformed JSON, or path ID `0` | 400 |
| `POST /products/99999/details` with a valid body | 404 |
| `POST /products/2/details` with `X-Simulate-Error: true` | 500 |

Valid body:
```json
{"product_id": 2, "sku": "ABC-123-XYZ", "manufacturer": "Acme Corporation", "category_id": 456, "weight": 1250, "some_other_id": 789}
```

All 29 tests pass with `go run` and inside Docker (server log on the left, Postman results on the right):

![Postman run against go run](screenshots/p2-postman-go-run.png)
![Postman run against Docker](screenshots/p2-postman-docker.png)

## Load testing (Locust)

```
cd locust
TARGET_HOST=http://<PUBLIC-IP>:8080 docker compose up --scale worker=4
LOCUSTFILE=locustfile_fast.py TARGET_HOST=http://<PUBLIC-IP>:8080 docker compose up --scale worker=4
```
Then open http://localhost:8089.

**Test design:** GET:POST = 9:1 because a store is read-heavy; product IDs are random in 1–1000; no `wait_time`, so each user sends back to back; 4 Locust workers on the client; target is one Fargate task (0.25 vCPU, 512 MB).

| Test | Users (spawn/s) | RPS | p50 | p95 | p99 | Failures | ECS CPU |
|---|---|---|---|---|---|---|---|
| Baseline, HttpUser | 1 (1) | 36 | 27 ms | 29 ms | 32 ms | 0 | ~0% |
| Stress, HttpUser | 500 (5) | 2,649 | 110 ms | 300 ms | 390 ms | 0 | ~100% |
| Stress, FastHttpUser | 500 (5) | 2,672 | 110 ms | 300 ms | 400 ms | 0 | ~100% |

Percentiles are over the whole run, ramp included; at 500 users the steady-state p50 is about 190 ms.

**Where it struggles:** RPS stops rising at about 2,650 from roughly 350 users, while ECS CPU sits at 100%. Extra users only queue: 500 users / 2,650 RPS ≈ 190 ms, which matches the steady-state p50 (Little's Law), and no requests fail.

**HttpUser vs FastHttpUser:** no difference, because the server is the bottleneck. FastHttpUser uses less client CPU per request, which only matters when Locust itself is the limit.

**Read-heavy tradeoff:** most traffic is lookups by ID, so a map gives O(1) reads and an RWMutex lets GETs run in parallel. GET and POST latencies are the same under load, so the 10% writes aren't causing lock contention; requests queue on CPU.

Baseline:
![1 user charts](screenshots/p4-1user-charts.png)
![1 user stats](screenshots/p4-1user-stats.png)

Stress test, HttpUser:
![500 users HttpUser charts](screenshots/p4-500users-http-charts.png)
![500 users HttpUser stats](screenshots/p4-500users-http-stats.png)

Stress test, FastHttpUser:
![500 users FastHttpUser charts](screenshots/p4-500users-fast-charts.png)
![500 users FastHttpUser stats](screenshots/p4-500users-fast-stats.png)

ECS CPU (CloudWatch, UTC): the two spikes at 03:31 and 03:54 are the HttpUser and FastHttpUser runs.
![ECS CPU during both stress tests](screenshots/p4-500users-both-ecs-cpu.png)

## Observability

TODO (Part V): Grafana dashboard (rate, errors, duration) idle vs loaded, request log excerpt, and when to use the dashboard vs the logs.
