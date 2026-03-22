# Order Load Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a reproducible single-node full-path order load baseline by adding partition-aware Kafka consumption, perf-only timeout and delay overrides, minimal Prometheus observability, and repeatable preparation/load scripts.

**Architecture:** Keep the existing async order-create contract intact, but widen the perf environment guardrails and remove the artificial single-partition single-consumer bottleneck. The implementation adds Kafka partition and worker knobs, runs one Kafka reader per worker in a shared consumer group, provides perf-specific config files, and adds scripts to prepare the topic, prewarm ledgers, and drive gateway-based load.

**Tech Stack:** Go, go-zero gateway/rest/zrpc, segmentio/kafka-go, MySQL, Redis, bash, k6, Prometheus

---

## File Map

### Existing files to modify

- `services/order-rpc/internal/config/config.go`
  Add `TopicPartitions` and `ConsumerWorkers` to `KafkaConfig`.
- `services/order-rpc/internal/mq/topic_admin.go`
  Stop hardcoding `NumPartitions: 1`; read desired partitions from config and expose current-partition inspection logic.
- `services/order-rpc/internal/mq/consumer.go`
  Split single-consumer construction from worker startup and make reader creation reusable per worker.
- `services/order-rpc/internal/svc/service_context.go`
  Replace single consumer instance wiring with a reusable consumer factory.
- `services/order-rpc/internal/logic/order_create_consumer_runner.go`
  Start `ConsumerWorkers` independent worker loops and stop them cleanly.
- `services/order-rpc/tests/integration/service_context_kafka_test.go`
  Assert topic creation behavior and Kafka config wiring for partitions/workers.
- `services/order-rpc/tests/integration/order_startup_test.go`
  Assert multiple workers start, restart, and stop correctly.
- `services/order-rpc/tests/integration/order_test_helpers_test.go`
  Replace the single fake consumer with a fake consumer factory that can mint many fake consumers.
- `services/gateway-api/etc/gateway-api.yaml`
  Add `Prometheus` section to the base config if not already present.
- `services/order-api/etc/order-api.yaml`
  Add `Prometheus` section to the base config.
- `services/order-rpc/etc/order-rpc.yaml`
  Add `Prometheus`, `TopicPartitions`, and `ConsumerWorkers` to the base config with conservative defaults.
- `services/gateway-api/tests/config/config_test.go`
  Extend config loading coverage to include Prometheus and upstream timeout expectations.
- `README.md`
  Document perf configs, Prometheus endpoints, topic preparation, and perf scripts.

### New files to create

- `services/order-rpc/etc/order-rpc.perf.yaml`
  Perf-only overrides: `MaxMessageDelay=60s`, `TopicPartitions=4`, `ConsumerWorkers=4`, Prometheus endpoint.
- `services/gateway-api/etc/gateway-api.perf.yaml`
  Perf-only overrides: `order-api Timeout=10000` and Prometheus endpoint.
- `services/order-api/etc/order-api.perf.yaml`
  Perf-only Prometheus config if base config should stay minimal.
- `services/order-api/tests/config/config_test.go`
  Config loading test for `order-api` Prometheus settings.
- `services/order-rpc/tests/integration/topic_admin_test.go`
  Dedicated tests for topic partition behavior and partition-count inspection helpers.
- `scripts/perf/prepare_order_kafka_topic.sh`
  Expand `order.create` to the configured partition count before a run.
- `scripts/perf/prewarm_order_ledgers.sh`
  Prime seat and purchase-limit ledgers before formal load.
- `scripts/perf/order_create_gateway_baseline.js`
  k6 script for `/order/create` through `gateway-api`.
- `scripts/perf/order_create_gateway_baseline.env.example`
  Template for JWT, IDs, and host variables used by the k6 script.
- `scripts/perf/collect_order_baseline.sh`
  Helper script to snapshot lag, row counts, and sampled visibility latency after a run.

## Task 1: Add Kafka Perf Knobs and Parse Them Safely

**Files:**
- Modify: `services/order-rpc/internal/config/config.go`
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Create: `services/order-rpc/etc/order-rpc.perf.yaml`
- Test: `services/order-rpc/tests/integration/service_context_kafka_test.go`

- [ ] **Step 1: Write the failing config test**

Add assertions that a test config can load:

```go
Kafka: config.KafkaConfig{
    TopicOrderCreate: "order.create.command.test",
    TopicPartitions:  4,
    ConsumerWorkers:  4,
    MaxMessageDelay:  60 * time.Second,
}
```

and that omitted values still default to:

```go
TopicPartitions: 1
ConsumerWorkers: 1
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestNewOrderServiceContext(BuildsKafkaProducer|EnsuresKafkaTopicExists)'
```

Expected: compile or assertion failure because `TopicPartitions` and `ConsumerWorkers` do not exist yet.

- [ ] **Step 3: Add the minimal config fields**

Update `KafkaConfig` to include:

```go
TopicPartitions int `json:",default=1"`
ConsumerWorkers int `json:",default=1"`
```

and keep the default behavior backward-compatible.

- [ ] **Step 4: Add base and perf YAML values**

In base config keep conservative defaults:

```yaml
Kafka:
  TopicPartitions: 1
  ConsumerWorkers: 1
```

In perf config override:

```yaml
Kafka:
  MaxMessageDelay: 60s
  TopicPartitions: 4
  ConsumerWorkers: 4
```

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestNewOrderServiceContext(BuildsKafkaProducer|EnsuresKafkaTopicExists)'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/order-rpc/internal/config/config.go services/order-rpc/etc/order-rpc.yaml services/order-rpc/etc/order-rpc.perf.yaml services/order-rpc/tests/integration/service_context_kafka_test.go
git commit -m "feat(order-rpc): add perf kafka worker config"
```

## Task 2: Make Topic Creation Partition-Aware Without Hidden Expansion

**Files:**
- Modify: `services/order-rpc/internal/mq/topic_admin.go`
- Create: `services/order-rpc/tests/integration/topic_admin_test.go`
- Modify: `services/order-rpc/tests/integration/service_context_kafka_test.go`

- [ ] **Step 1: Write the failing topic-admin tests**

Add tests for three cases:

```go
func TestEnsureOrderCreateTopicCreatesConfiguredPartitions(t *testing.T)
func TestEnsureOrderCreateTopicLeavesExistingTopicWhenPartitionsAreLower(t *testing.T)
func TestGetOrderCreateTopicPartitionCount(t *testing.T)
```

The key assertion for creation should be:

```go
if got != 4 {
    t.Fatalf("expected 4 partitions, got %d", got)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestEnsureOrderCreateTopic|TestGetOrderCreateTopicPartitionCount'
```

Expected: FAIL because topic creation is hardcoded to `1` and there is no partition-count helper.

- [ ] **Step 3: Implement partition-aware topic creation**

Update `EnsureOrderCreateTopic` to:

- derive desired partitions from config
- create missing topics with that count
- not auto-expand existing topics
- expose a helper like:

```go
func OrderCreateTopicPartitionCount(cfg config.KafkaConfig) (int, error)
```

If existing partitions are lower than desired, log a warning from the caller rather than mutating Kafka metadata.

- [ ] **Step 4: Wire service-context warnings**

After `EnsureOrderCreateTopic`, read the current partition count and log a warning if:

```go
current < cfg.Kafka.TopicPartitions
```

without aborting startup.

- [ ] **Step 5: Run the tests to verify they pass**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestEnsureOrderCreateTopic|TestGetOrderCreateTopicPartitionCount|TestNewOrderServiceContextEnsuresKafkaTopicExists'
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/order-rpc/internal/mq/topic_admin.go services/order-rpc/internal/svc/service_context.go services/order-rpc/tests/integration/topic_admin_test.go services/order-rpc/tests/integration/service_context_kafka_test.go
git commit -m "feat(order-rpc): make order topic partitions configurable"
```

## Task 3: Replace the Single Consumer With a Worker-Based Consumer Factory

**Files:**
- Modify: `services/order-rpc/internal/mq/consumer.go`
- Modify: `services/order-rpc/internal/svc/service_context.go`
- Modify: `services/order-rpc/internal/logic/order_create_consumer_runner.go`
- Modify: `services/order-rpc/tests/integration/order_startup_test.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`
- Test: `services/order-rpc/tests/integration/create_order_consumer_logic_test.go`

- [ ] **Step 1: Write the failing startup tests**

Replace the single-consumer expectation with worker-aware assertions:

```go
if factory.createCalls != 4 {
    t.Fatalf("expected 4 consumers, got %d", factory.createCalls)
}
if factory.closeCalls != 4 {
    t.Fatalf("expected 4 closes, got %d", factory.closeCalls)
}
```

Also cover restart behavior for one worker:

```go
if consumers[0].startCalls < 2 {
    t.Fatalf("expected worker restart after recoverable error")
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestKafkaConsumer(StartAndClose|RestartsAfterRecoverableStartError)'
```

Expected: FAIL because the runner only manages one consumer instance.

- [ ] **Step 3: Introduce a factory abstraction**

Refactor the MQ layer to expose:

```go
type OrderCreateConsumerFactory interface {
    New(cfg config.KafkaConfig) OrderCreateConsumer
}
```

and let `ServiceContext` keep:

```go
OrderCreateConsumerFactory mq.OrderCreateConsumerFactory
```

instead of a single prebuilt consumer.

- [ ] **Step 4: Implement one-reader-per-worker startup**

In `StartOrderCreateConsumer`:

- normalize `ConsumerWorkers <= 0` to `1`
- start one goroutine per worker
- create one reader per worker via the factory
- restart each worker independently after recoverable errors
- close every worker on stop

The control flow should look like:

```go
for workerID := 0; workerID < workers; workerID++ {
    consumer := factory.New(cfg)
    go runWorker(workerID, consumer)
}
```

- [ ] **Step 5: Keep consumer logic unchanged**

Do not change `CreateOrderConsumerLogic.Consume`; the refactor is only about startup topology, not message semantics.

- [ ] **Step 6: Run tests to verify they pass**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration -run 'TestKafkaConsumer(StartAndClose|RestartsAfterRecoverableStartError|CreateOrderConsumer)'
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add services/order-rpc/internal/mq/consumer.go services/order-rpc/internal/svc/service_context.go services/order-rpc/internal/logic/order_create_consumer_runner.go services/order-rpc/tests/integration/order_startup_test.go services/order-rpc/tests/integration/order_test_helpers_test.go
git commit -m "feat(order-rpc): run order create consumers with worker fanout"
```

## Task 4: Add Perf-Specific Gateway and go-zero Prometheus Config

**Files:**
- Modify: `services/gateway-api/etc/gateway-api.yaml`
- Create: `services/gateway-api/etc/gateway-api.perf.yaml`
- Modify: `services/order-api/etc/order-api.yaml`
- Create: `services/order-api/etc/order-api.perf.yaml`
- Modify: `services/order-rpc/etc/order-rpc.yaml`
- Modify: `services/gateway-api/tests/config/config_test.go`
- Create: `services/order-api/tests/config/config_test.go`

- [ ] **Step 1: Write the failing config tests**

Extend the gateway config test to assert:

```go
if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
    t.Fatalf("expected prometheus config to load")
}
```

and add a new `order-api` config test with a fixture containing:

```yaml
Prometheus:
  Host: 0.0.0.0
  Port: 9102
```

- [ ] **Step 2: Run the tests to verify they fail**

Run:

```bash
go test -count=1 ./services/gateway-api/tests/config ./services/order-api/tests/config
```

Expected: FAIL because `order-api/tests/config` does not exist yet and the fixture/config pairs are incomplete.

- [ ] **Step 3: Add Prometheus to service YAML**

Add Prometheus blocks to the base YAMLs using fixed ports that do not collide with the app ports. Example:

```yaml
Prometheus:
  Host: 0.0.0.0
  Port: 9101
```

Use separate ports per service, and in `gateway-api.perf.yaml` set:

```yaml
Upstreams:
  - Name: order-api
    Http:
      Timeout: 10000
```

- [ ] **Step 4: Add perf-only YAML files**

Create perf override files rather than mutating the base runtime assumptions. Keep the files focused on perf overrides only.

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
go test -count=1 ./services/gateway-api/tests/config ./services/order-api/tests/config
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/gateway-api/etc/gateway-api.yaml services/gateway-api/etc/gateway-api.perf.yaml services/order-api/etc/order-api.yaml services/order-api/etc/order-api.perf.yaml services/order-rpc/etc/order-rpc.yaml services/gateway-api/tests/config/config_test.go services/order-api/tests/config/config_test.go
git commit -m "feat(config): add perf and prometheus config for order baseline"
```

## Task 5: Add Kafka Preparation and Ledger Prewarm Scripts

**Files:**
- Create: `scripts/perf/prepare_order_kafka_topic.sh`
- Create: `scripts/perf/prewarm_order_ledgers.sh`
- Modify: `README.md`

- [ ] **Step 1: Write the failing script smoke-checks**

Create shell smoke-check expectations in the plan and validate manually with:

```bash
bash -n scripts/perf/prepare_order_kafka_topic.sh
bash -n scripts/perf/prewarm_order_ledgers.sh
```

The scripts do not exist yet, so the check should fail.

- [ ] **Step 2: Run the checks to verify they fail**

Run:

```bash
bash -n scripts/perf/prepare_order_kafka_topic.sh
```

Expected: `No such file or directory`.

- [ ] **Step 3: Implement the topic preparation script**

The script should:

- read broker, topic, and partition count from env with sane defaults
- inspect current topic partitions
- create or expand the topic explicitly before a test run

Core command shape:

```bash
kafka-topics.sh --bootstrap-server "${KAFKA_BOOTSTRAP}" --alter --topic "${TOPIC}" --partitions "${PARTITIONS}"
```

- [ ] **Step 4: Implement the ledger prewarm script**

The script should:

- call a small warm-up batch of `/order/create`
- then poll sampled `/order/get` visibility until warm
- fail fast if ledger-not-ready or order-not-visible patterns persist

Keep it parameterized by:

```bash
GATEWAY_BASE_URL
JWT
PROGRAM_ID
TICKET_CATEGORY_ID
TICKET_USER_IDS
```

- [ ] **Step 5: Run shell validation**

Run:

```bash
bash -n scripts/perf/prepare_order_kafka_topic.sh
bash -n scripts/perf/prewarm_order_ledgers.sh
```

Expected: both PASS with no syntax errors.

- [ ] **Step 6: Update README**

Add a short perf section describing:

- perf config startup commands
- topic preparation
- ledger prewarm
- Prometheus scrape endpoints

- [ ] **Step 7: Commit**

```bash
git add scripts/perf/prepare_order_kafka_topic.sh scripts/perf/prewarm_order_ledgers.sh README.md
git commit -m "feat(perf): add order baseline preparation scripts"
```

## Task 6: Add the Gateway-Based Load Script and Result Collection Helper

**Files:**
- Create: `scripts/perf/order_create_gateway_baseline.js`
- Create: `scripts/perf/order_create_gateway_baseline.env.example`
- Create: `scripts/perf/collect_order_baseline.sh`
- Modify: `README.md`

- [ ] **Step 1: Write the failing script validation steps**

Add validation expectations:

```bash
test -f scripts/perf/order_create_gateway_baseline.js
test -f scripts/perf/collect_order_baseline.sh
```

These should fail before implementation.

- [ ] **Step 2: Run the checks to verify they fail**

Run:

```bash
test -f scripts/perf/order_create_gateway_baseline.js
```

Expected: non-zero exit.

- [ ] **Step 3: Implement the k6 load script**

The script should:

- target `POST /order/create` through `gateway-api`
- inject JWT and JSON payload
- load test data from env
- emit thresholds for:

```js
http_req_failed: ['rate<0.01']
http_req_duration: ['p(99)<10000']
```

Use scenario names that match the baseline stages:

```js
warmup
steady_state
```

- [ ] **Step 4: Implement the result collector**

The collector should gather:

- Kafka lag snapshot
- `d_order` row delta
- sampled `/order/get` visibility latency
- optional grep count for `skip expired order create event`

The script can shell out to:

```bash
mysql
curl
docker exec kafka ...
```

and print a compact summary table.

- [ ] **Step 5: Validate script syntax**

Run:

```bash
bash -n scripts/perf/collect_order_baseline.sh
node -c scripts/perf/order_create_gateway_baseline.js
```

Expected: both PASS.

- [ ] **Step 6: Update README**

Document:

- how to run the k6 script
- how to supply env vars
- how to run the result collector
- what constitutes a pass for the first baseline

- [ ] **Step 7: Commit**

```bash
git add scripts/perf/order_create_gateway_baseline.js scripts/perf/order_create_gateway_baseline.env.example scripts/perf/collect_order_baseline.sh README.md
git commit -m "feat(perf): add gateway order baseline load script"
```

## Task 7: Run Focused Verification and Capture the Baseline Checklist

**Files:**
- Modify: `README.md`
- Optional notes: `docs/api/order-checkout-acceptance.md`

- [ ] **Step 1: Run focused Go tests**

Run:

```bash
go test -count=1 ./services/order-rpc/tests/integration ./services/gateway-api/tests/config ./services/order-api/tests/config
```

Expected: PASS.

- [ ] **Step 2: Run config boot smoke tests**

Run:

```bash
go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.perf.yaml
go run services/order-api/order.go -f services/order-api/etc/order-api.perf.yaml
go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.perf.yaml
```

Expected: each service starts with no config parse error. Stop them after boot verification.

- [ ] **Step 3: Run the prep scripts**

Run:

```bash
bash scripts/perf/prepare_order_kafka_topic.sh
bash scripts/perf/prewarm_order_ledgers.sh
```

Expected: topic partition count reaches target and warm-up orders become visible.

- [ ] **Step 4: Run a low-RPS baseline sample**

Run:

```bash
k6 run -e ENV_FILE=scripts/perf/order_create_gateway_baseline.env.example scripts/perf/order_create_gateway_baseline.js
```

Expected: no expired skips, low failure rate, and acceptable p99 latency.

- [ ] **Step 5: Capture the operator checklist**

Add a compact checklist to `README.md` covering:

- start infra
- start perf configs
- prepare topic
- prewarm ledgers
- run k6
- collect baseline report

- [ ] **Step 6: Commit**

```bash
git add README.md docs/api/order-checkout-acceptance.md
git commit -m "docs: document order load baseline verification flow"
```

## Notes for Execution

- Use `@superpowers:test-driven-development` for each code task before implementation changes.
- Use `@superpowers:verification-before-completion` before claiming the baseline tooling is ready.
- Do not expand the scope to DLQ, retry pipelines, or multi-instance deployment in this implementation pass.
- If k6 is unavailable locally, stop and decide whether to add installation instructions or swap to another tool before implementing the load script.
