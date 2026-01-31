# Service-Feature Operations Guide

## Overview

This guide covers day-2 operations for Service-Feature, including monitoring, alerting, troubleshooting, maintenance, and scaling procedures.

---

## Monitoring

### Key Metrics

#### Feature Execution Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `feature_executions_total` | Counter | Total feature executions by state/outcome | - |
| `feature_execution_duration_seconds` | Histogram | Feature execution duration by state | P99 > 30m |
| `feature_executions_active` | Gauge | Currently active features | > capacity |
| `feature_steps_total` | Counter | Steps executed by type/outcome | - |
| `feature_step_duration_seconds` | Histogram | Step execution duration | P99 > 5m |

#### Event Processing Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `event_bus_consumer_lag` | Gauge | Events waiting to be processed | > 1000 |
| `event_processing_duration_seconds` | Histogram | Event processing time | P99 > 10s |
| `event_processing_errors_total` | Counter | Event processing failures | Rate > 5/min |

#### Git Operations Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `git_clone_duration_seconds` | Histogram | Clone operation duration | P99 > 5m |
| `git_push_duration_seconds` | Histogram | Push operation duration | P99 > 1m |
| `git_operations_total` | Counter | Git operations by type/outcome | - |
| `git_operations_errors_total` | Counter | Git operation failures | Rate > 10/min |

#### LLM Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `llm_request_duration_seconds` | Histogram | LLM request latency | P99 > 60s |
| `llm_tokens_total` | Counter | Tokens used (input/output) | - |
| `llm_requests_total` | Counter | LLM requests by function/outcome | - |
| `llm_rate_limit_hits_total` | Counter | Rate limit encounters | > 10/min |

#### Sandbox Metrics

| Metric | Type | Description | Alert Threshold |
|--------|------|-------------|-----------------|
| `sandbox_execution_duration_seconds` | Histogram | Sandbox execution time | P99 > 10m |
| `sandbox_active` | Gauge | Active sandboxes | > node capacity |
| `sandbox_oom_kills_total` | Counter | OOM kill events | > 5/hour |

### Prometheus Queries

#### Feature Success Rate
```promql
# Success rate over last hour
sum(rate(feature_executions_total{outcome="completed"}[1h])) /
sum(rate(feature_executions_total[1h])) * 100
```

#### Average Feature Duration
```promql
# Average duration by state
histogram_quantile(0.5, rate(feature_execution_duration_seconds_bucket[1h]))
```

#### Consumer Lag
```promql
# Maximum consumer lag across all partitions
max(event_bus_consumer_lag) by (partition)
```

#### LLM Error Rate
```promql
# LLM error rate by function
sum(rate(llm_requests_total{outcome="error"}[5m])) by (function) /
sum(rate(llm_requests_total[5m])) by (function) * 100
```

### Grafana Dashboards

#### Executive Dashboard
- Feature execution count (hourly/daily)
- Success/failure rate
- Average completion time
- Active features

#### Operations Dashboard
- Event processing lag
- Worker utilization
- Git operation latency
- LLM token consumption

#### Infrastructure Dashboard
- CPU/memory by component
- Disk I/O
- Network traffic
- Pod health

---

## Alerting

### Critical Alerts

```yaml
# alerting-rules.yaml
groups:
- name: feature-platform-critical
  rules:
  - alert: FeatureWorkerDown
    expr: up{job="feature-worker"} == 0
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Feature worker {{ $labels.pod }} is down"
      runbook: "https://wiki.example.com/runbooks/feature-worker-down"

  - alert: EventBusConsumerLagCritical
    expr: event_bus_consumer_lag > 5000
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Event consumer lag critical on partition {{ $labels.partition }}"
      runbook: "https://wiki.example.com/runbooks/consumer-lag"

  - alert: DatabaseConnectionPoolExhausted
    expr: pg_stat_activity_count / pg_stat_activity_max > 0.9
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "Database connection pool nearly exhausted"
      runbook: "https://wiki.example.com/runbooks/db-connections"

  - alert: FeatureExecutionStuck
    expr: |
      (time() - feature_execution_state_change_timestamp_seconds{state="executing"}) > 3600
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Feature {{ $labels.feature_id }} stuck in executing state"
      runbook: "https://wiki.example.com/runbooks/stuck-feature"
```

### Warning Alerts

```yaml
- name: feature-platform-warning
  rules:
  - alert: FeatureSuccessRateLow
    expr: |
      (sum(rate(feature_executions_total{outcome="completed"}[1h])) /
       sum(rate(feature_executions_total[1h]))) < 0.9
    for: 15m
    labels:
      severity: warning
    annotations:
      summary: "Feature success rate below 90%"
      description: "Current rate: {{ $value | humanizePercentage }}"

  - alert: LLMLatencyHigh
    expr: histogram_quantile(0.99, rate(llm_request_duration_seconds_bucket[5m])) > 60
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "LLM P99 latency exceeds 60 seconds"

  - alert: GitCloneSlowdown
    expr: histogram_quantile(0.95, rate(git_clone_duration_seconds_bucket[15m])) > 300
    for: 10m
    labels:
      severity: warning
    annotations:
      summary: "Git clone P95 latency exceeds 5 minutes"

  - alert: SandboxOOMFrequent
    expr: rate(sandbox_oom_kills_total[1h]) > 5
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Frequent sandbox OOM kills"
```

---

## Troubleshooting

### Common Issues

#### Issue: Features Stuck in Pending State

**Symptoms:**
- Features remain in PENDING state for extended periods
- Consumer lag increasing

**Diagnosis:**
```bash
# Check consumer group status
kubectl exec -it redpanda-0 -n feature-system -- rpk group describe feature-workers

# Check worker logs for errors
kubectl logs -l app=feature-worker -n feature-system --tail=100 | grep -i error

# Check for partition assignment
kubectl logs -l app=feature-worker -n feature-system | grep "partition assigned"
```

**Resolution:**
1. Verify workers are running and healthy
2. Check for rebalancing issues
3. Restart workers if necessary: `kubectl rollout restart statefulset/feature-worker -n feature-system`

---

#### Issue: Git Clone Failures

**Symptoms:**
- Features fail in ANALYZING state
- `repository.clone.failed` events

**Diagnosis:**
```bash
# Check git service logs
kubectl logs -l app=git-service -n feature-system --tail=100

# Test connectivity from worker
kubectl exec -it feature-worker-0 -n feature-system -- \
  git ls-remote --exit-code git@github.com:org/repo.git

# Check credential validity
kubectl exec -it feature-worker-0 -n feature-system -- \
  curl -s -H "Authorization: Bearer $TOKEN" https://api.github.com/user
```

**Resolution:**
1. Verify repository URL is correct
2. Check credential validity and refresh if needed
3. Verify network egress rules allow git remote access
4. Check DNS resolution

---

#### Issue: LLM Rate Limiting

**Symptoms:**
- 429 errors in LLM orchestrator logs
- Features stuck in PLANNING or EXECUTING state
- `llm_rate_limit_hits_total` increasing

**Diagnosis:**
```bash
# Check LLM orchestrator logs
kubectl logs -l app=llm-orchestrator -n feature-system | grep "rate limit"

# Check rate limit metrics
kubectl exec -it prometheus-0 -n observability -- \
  promtool query instant 'sum(rate(llm_rate_limit_hits_total[5m]))'
```

**Resolution:**
1. Scale LLM orchestrator instances
2. Increase backoff delays in configuration
3. Contact LLM provider to increase limits
4. Implement request queuing

---

#### Issue: Sandbox OOM Kills

**Symptoms:**
- Features fail in VERIFYING state
- `sandbox_oom_kills_total` increasing
- Build/test failures with exit code 137

**Diagnosis:**
```bash
# Check sandbox manager logs
kubectl logs -l app=sandbox-manager -n feature-system | grep -i oom

# Check container events
kubectl get events -n feature-system --field-selector reason=OOMKilled

# Check memory usage
kubectl top pods -l app=sandbox-manager -n feature-system
```

**Resolution:**
1. Increase sandbox memory limits
2. Optimize build processes (reduce parallelism)
3. Check for memory leaks in tests
4. Scale out sandbox nodes

---

#### Issue: Database Connection Pool Exhaustion

**Symptoms:**
- "too many connections" errors
- Service timeouts
- Slow queries

**Diagnosis:**
```bash
# Check active connections
kubectl exec -it postgresql-0 -n feature-system -- \
  psql -c "SELECT count(*) FROM pg_stat_activity;"

# Check connection sources
kubectl exec -it postgresql-0 -n feature-system -- \
  psql -c "SELECT application_name, count(*) FROM pg_stat_activity GROUP BY 1;"

# Check for idle connections
kubectl exec -it postgresql-0 -n feature-system -- \
  psql -c "SELECT * FROM pg_stat_activity WHERE state = 'idle' AND query_start < now() - interval '10 minutes';"
```

**Resolution:**
1. Tune connection pool size in services
2. Add PgBouncer for connection pooling
3. Identify and fix connection leaks
4. Scale database if needed

---

### Debug Commands

#### View Feature Execution Details
```bash
# Get feature from API
curl -s -X POST https://feature.api.example.com/feature.feature.v1.FeatureService/Get \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"id": "feature-uuid"}' | jq

# Query database directly
kubectl exec -it postgresql-0 -n feature-system -- \
  psql -c "SELECT id, state, error FROM feature_executions WHERE id = 'feature-uuid';"
```

#### View Event Stream
```bash
# Consume events for a feature
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk topic consume feature.events --offset start --num 100 | \
  jq 'select(.feature_execution_id == "feature-uuid")'
```

#### Check Worker Assignment
```bash
# View worker partition assignments
kubectl logs feature-worker-0 -n feature-system | grep "partition"

# Check consumer group membership
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk group describe feature-workers
```

---

## Maintenance

### Routine Tasks

#### Daily
- [ ] Review alert dashboard
- [ ] Check consumer lag metrics
- [ ] Verify backup completion
- [ ] Review failed feature executions

#### Weekly
- [ ] Review capacity metrics
- [ ] Check certificate expiration
- [ ] Review audit logs
- [ ] Clean up expired artifacts

#### Monthly
- [ ] Rotate credentials
- [ ] Review and update alerts
- [ ] Capacity planning review
- [ ] Security patch review

### Database Maintenance

#### Vacuum and Analyze
```sql
-- Run periodically for optimal performance
VACUUM ANALYZE feature_executions;
VACUUM ANALYZE execution_steps;
VACUUM ANALYZE execution_events;
```

#### Index Maintenance
```sql
-- Reindex if needed
REINDEX INDEX CONCURRENTLY idx_feature_created;
REINDEX INDEX CONCURRENTLY idx_event_feature_seq;
```

#### Archive Old Data
```sql
-- Archive completed features older than 90 days
INSERT INTO feature_executions_archive
SELECT * FROM feature_executions
WHERE completed_at < NOW() - INTERVAL '90 days'
  AND state IN (6, 7, 8); -- completed, failed, cancelled

DELETE FROM feature_executions
WHERE completed_at < NOW() - INTERVAL '90 days'
  AND state IN (6, 7, 8);
```

### Event Bus Maintenance

#### Topic Compaction
```bash
# Check topic retention
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk topic describe feature.events

# Adjust retention if needed
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk topic alter-config feature.events --set retention.ms=604800000
```

#### Consumer Group Reset
```bash
# Reset consumer group offset (USE WITH CAUTION)
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk group seek feature-workers --to-latest
```

### Artifact Cleanup

```bash
# Clean up expired artifacts
kubectl create job artifact-cleanup --from=cronjob/artifact-cleanup -n feature-system

# Manual cleanup
aws s3 ls s3://feature-artifacts/ --recursive | \
  awk '{print $4}' | \
  while read key; do
    # Check if artifact is expired
    aws s3api head-object --bucket feature-artifacts --key "$key" | \
      jq -r '.Metadata.expires_at' | \
      xargs -I {} test {} -lt $(date +%s) && \
      aws s3 rm "s3://feature-artifacts/$key"
  done
```

---

## Scaling

### Horizontal Scaling

#### Scale Workers
```bash
# Scale to match partition count
PARTITION_COUNT=$(kubectl exec -it redpanda-0 -n feature-system -- \
  rpk topic describe feature.events -f json | jq '.partitions | length')

kubectl scale statefulset/feature-worker --replicas=$PARTITION_COUNT -n feature-system
```

#### Scale API Services
```bash
# Scale based on request volume
kubectl scale deployment/feature-service --replicas=6 -n feature-system
```

### Vertical Scaling

#### Increase Worker Resources
```bash
kubectl patch statefulset feature-worker -n feature-system --type='json' -p='[
  {"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/memory", "value": "16Gi"},
  {"op": "replace", "path": "/spec/template/spec/containers/0/resources/limits/cpu", "value": "8"}
]'
```

### Adding Event Bus Partitions

```bash
# Add partitions (cannot be reduced)
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk topic alter-config feature.events --set partitions=128

# Scale workers to match
kubectl scale statefulset/feature-worker --replicas=128 -n feature-system
```

---

## Disaster Recovery

### Backup Procedures

#### Database Backup
```bash
# Manual backup
kubectl exec -it postgresql-0 -n feature-system -- \
  pg_dump -Fc feature_db > backup-$(date +%Y%m%d).dump

# Upload to S3
aws s3 cp backup-$(date +%Y%m%d).dump s3://backups/feature/
```

#### Event Store Backup
```bash
# Kafka topic backup using MirrorMaker or similar
# Configured via Redpanda operator
```

### Recovery Procedures

#### Database Recovery
```bash
# Restore from backup
kubectl exec -it postgresql-0 -n feature-system -- \
  pg_restore -d feature_db backup.dump
```

#### Feature State Recovery
```bash
# Replay events to rebuild state
# Events are replayed automatically on worker restart

# Force replay for specific feature
kubectl exec -it feature-worker-0 -n feature-system -- \
  /feature-worker replay --feature-id=<id> --from-sequence=0
```

### Failover Procedures

#### Database Failover
```bash
# Patroni handles automatic failover
# Manual switchover if needed
kubectl exec -it postgresql-0 -n feature-system -- \
  patronictl switchover
```

#### Multi-Region Failover
1. Update DNS to point to secondary region
2. Promote secondary database
3. Verify event store replication
4. Scale up secondary workers

---

## Runbooks

### Runbook: Worker Not Processing Events

**Trigger:** `FeatureWorkerDown` alert

**Steps:**
1. Check pod status: `kubectl get pods -l app=feature-worker -n feature-system`
2. Check pod logs: `kubectl logs feature-worker-0 -n feature-system --tail=50`
3. Check consumer group: `rpk group describe feature-workers`
4. If pod is CrashLoopBackOff, check events: `kubectl describe pod feature-worker-0 -n feature-system`
5. If necessary, delete pod to trigger reschedule: `kubectl delete pod feature-worker-0 -n feature-system`
6. Verify recovery: `kubectl logs feature-worker-0 -n feature-system | grep "started"`

### Runbook: High Consumer Lag

**Trigger:** `EventBusConsumerLagCritical` alert

**Steps:**
1. Check lag by partition: `rpk group describe feature-workers`
2. Identify slow partitions
3. Check worker logs for errors on those partitions
4. Scale workers if needed: `kubectl scale statefulset/feature-worker --replicas=<n>`
5. If single feature is blocking, consider cancelling it
6. Monitor lag decreasing

### Runbook: Feature Stuck

**Trigger:** `FeatureExecutionStuck` alert

**Steps:**
1. Get feature details from API
2. Check last event in event store
3. Check worker logs for errors
4. If worker crashed during execution:
   - Feature will auto-resume on worker restart
   - Verify worker is assigned to partition
5. If LLM call is hanging:
   - Check LLM orchestrator logs
   - Consider cancelling feature if needed
6. If sandbox is stuck:
   - Check sandbox manager logs
   - Force cleanup if necessary

---

## SLA Targets

| Metric | Target | Measurement |
|--------|--------|-------------|
| Feature completion rate | > 95% | Successful / Total |
| Feature latency (P50) | < 10 min | API to completion |
| Feature latency (P99) | < 30 min | API to completion |
| API availability | 99.9% | Successful responses |
| Event processing lag | < 1000 | Maximum lag |

---

## Contact Information

| Role | Contact |
|------|---------|
| On-Call Engineer | oncall@example.com |
| Platform Team | platform-team@example.com |
| Security Team | security@example.com |
| Escalation | platform-lead@example.com |
