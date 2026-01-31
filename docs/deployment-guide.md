# Service-Feature Deployment Guide

## Overview

This guide covers deploying Service-Feature in a production Kubernetes environment. The platform is designed for horizontal scalability, high availability, and operational observability.

---

## Prerequisites

### Infrastructure Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| Kubernetes | 1.28+ | 1.29+ |
| PostgreSQL | 15+ | 16+ (HA cluster) |
| Kafka/Redpanda | 3.5+ | Redpanda 23.3+ |
| Vault | 1.15+ | 1.16+ (HA) |
| S3-compatible storage | Any | MinIO or cloud-native |

### Resource Requirements

| Component | CPU | Memory | Storage | Replicas |
|-----------|-----|--------|---------|----------|
| API Gateway | 1 core | 1 GB | - | 2-4 |
| Feature Service | 2 cores | 2 GB | - | 2-4 |
| Feature Worker | 4 cores | 8 GB | 50 GB | 4-16 |
| Git Service | 2 cores | 4 GB | 100 GB cache | 2-4 |
| LLM Orchestrator | 2 cores | 2 GB | - | 2-4 |
| Sandbox Manager | 2 cores | 4 GB | 100 GB | 1 per node |
| PostgreSQL | 4 cores | 16 GB | 500 GB SSD | 3 (HA) |
| Kafka/Redpanda | 4 cores | 16 GB | 500 GB SSD | 3 (HA) |

---

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                        PRODUCTION DEPLOYMENT                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │                    INGRESS (Load Balancer)                               │    │
│  │                                                                          │    │
│  │  • TLS termination                                                       │    │
│  │  • Rate limiting                                                         │    │
│  │  • DDoS protection                                                       │    │
│  └─────────────────────────────────────────────────────────────────────────┘    │
│                                     │                                            │
│                                     ▼                                            │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │
│  │              KUBERNETES CLUSTER (feature-system namespace)               │    │
│  │                                                                          │    │
│  │  ┌───────────────────────────────────────────────────────────────────┐  │    │
│  │  │ CONTROL PLANE                                                      │  │    │
│  │  │                                                                    │  │    │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │  │    │
│  │  │  │ API Gateway │  │  Feature    │  │ Repository  │               │  │    │
│  │  │  │ (Deployment)│  │  Service    │  │  Service    │               │  │    │
│  │  │  │ replicas: 3 │  │ replicas: 3 │  │ replicas: 2 │               │  │    │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘               │  │    │
│  │  └───────────────────────────────────────────────────────────────────┘  │    │
│  │                                                                          │    │
│  │  ┌───────────────────────────────────────────────────────────────────┐  │    │
│  │  │ DATA PLANE                                                         │  │    │
│  │  │                                                                    │  │    │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │  │    │
│  │  │  │  Feature    │  │    Git      │  │    LLM      │               │  │    │
│  │  │  │  Workers    │  │  Service    │  │Orchestrator │               │  │    │
│  │  │  │(StatefulSet)│  │ (Deployment)│  │ (Deployment)│               │  │    │
│  │  │  │ replicas: 8 │  │ replicas: 3 │  │ replicas: 3 │               │  │    │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘               │  │    │
│  │  │                                                                    │  │    │
│  │  │  ┌─────────────┐                                                  │  │    │
│  │  │  │  Sandbox    │                                                  │  │    │
│  │  │  │  Manager    │                                                  │  │    │
│  │  │  │ (DaemonSet) │                                                  │  │    │
│  │  │  └─────────────┘                                                  │  │    │
│  │  └───────────────────────────────────────────────────────────────────┘  │    │
│  │                                                                          │    │
│  │  ┌───────────────────────────────────────────────────────────────────┐  │    │
│  │  │ PERSISTENCE                                                        │  │    │
│  │  │                                                                    │  │    │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │  │    │
│  │  │  │ PostgreSQL  │  │   Kafka/    │  │    MinIO    │               │  │    │
│  │  │  │   (HA)      │  │  Redpanda   │  │ (S3-compat) │               │  │    │
│  │  │  │ replicas: 3 │  │ replicas: 3 │  │ replicas: 4 │               │  │    │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘               │  │    │
│  │  └───────────────────────────────────────────────────────────────────┘  │    │
│  │                                                                          │    │
│  └──────────────────────────────────────────────────────────────────────────┘    │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Kubernetes Manifests

### Namespace and RBAC

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: feature-system
  labels:
    name: feature-system
    istio-injection: enabled

---
# service-account.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: feature-service
  namespace: feature-system

---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: feature-worker
  namespace: feature-system

---
# rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: feature-worker-role
  namespace: feature-system
rules:
- apiGroups: [""]
  resources: ["pods", "pods/exec"]
  verbs: ["create", "get", "list", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: feature-worker-binding
  namespace: feature-system
subjects:
- kind: ServiceAccount
  name: feature-worker
  namespace: feature-system
roleRef:
  kind: Role
  name: feature-worker-role
  apiGroup: rbac.authorization.k8s.io
```

### ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feature-config
  namespace: feature-system
data:
  # Event Bus
  EVENT_BUS_BROKERS: "redpanda-0.redpanda.feature-system.svc:9092,redpanda-1.redpanda.feature-system.svc:9092,redpanda-2.redpanda.feature-system.svc:9092"
  EVENT_BUS_TOPIC_PREFIX: "feature"
  EVENT_BUS_CONSUMER_GROUP: "feature-workers"
  EVENT_BUS_PARTITION_COUNT: "64"

  # Git Operations
  GIT_WORKSPACE_BASE_PATH: "/var/feature/workspaces"
  GIT_CLONE_TIMEOUT_SECONDS: "300"
  GIT_OPERATION_TIMEOUT_SECONDS: "60"

  # Sandbox
  SANDBOX_RUNTIME: "containerd"
  SANDBOX_CPU_LIMIT: "4"
  SANDBOX_MEMORY_LIMIT_MB: "8192"
  SANDBOX_TIMEOUT_SECONDS: "600"

  # BAML/LLM
  BAML_CLIENT_PROVIDER: "anthropic"
  BAML_CLIENT_MODEL: "claude-sonnet-4-20250514"
  BAML_MAX_TOKENS: "8192"
  BAML_TEMPERATURE: "0.1"

  # Observability
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector.observability.svc:4317"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
```

### Secrets

```yaml
# external-secrets.yaml (using External Secrets Operator)
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: feature-secrets
  namespace: feature-system
spec:
  refreshInterval: 1h
  secretStoreRef:
    kind: ClusterSecretStore
    name: vault-backend
  target:
    name: feature-secrets
    creationPolicy: Owner
  data:
  - secretKey: DATABASE_URL
    remoteRef:
      key: feature/database
      property: url
  - secretKey: BAML_CLIENT_API_KEY
    remoteRef:
      key: feature/anthropic
      property: api_key
  - secretKey: DEK_ACTIVE_ENCRYPTION_TOKEN
    remoteRef:
      key: feature/encryption
      property: active_key
  - secretKey: DEK_LOOKUP_TOKEN
    remoteRef:
      key: feature/encryption
      property: hmac_key
  - secretKey: OIDC_CLIENT_SECRET
    remoteRef:
      key: feature/oidc
      property: client_secret
```

### Feature Service Deployment

```yaml
# feature-service.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: feature-service
  namespace: feature-system
  labels:
    app: feature-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: feature-service
  template:
    metadata:
      labels:
        app: feature-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: feature-service
      containers:
      - name: feature-service
        image: registry.example.com/feature-service:latest
        ports:
        - name: http
          containerPort: 80
        - name: metrics
          containerPort: 9090
        envFrom:
        - configMapRef:
            name: feature-config
        - secretRef:
            name: feature-secrets
        env:
        - name: SERVICE_NAME
          value: "feature-service"
        resources:
          requests:
            cpu: "500m"
            memory: "512Mi"
          limits:
            cpu: "2000m"
            memory: "2Gi"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 80
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 80
          initialDelaySeconds: 5
          periodSeconds: 5
        securityContext:
          runAsNonRoot: true
          runAsUser: 65532
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: feature-service
              topologyKey: kubernetes.io/hostname

---
apiVersion: v1
kind: Service
metadata:
  name: feature-service
  namespace: feature-system
spec:
  selector:
    app: feature-service
  ports:
  - name: http
    port: 80
    targetPort: 80
  - name: metrics
    port: 9090
    targetPort: 9090
```

### Feature Worker StatefulSet

```yaml
# feature-worker.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: feature-worker
  namespace: feature-system
  labels:
    app: feature-worker
spec:
  serviceName: feature-worker
  replicas: 8
  podManagementPolicy: Parallel
  selector:
    matchLabels:
      app: feature-worker
  template:
    metadata:
      labels:
        app: feature-worker
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: feature-worker
      terminationGracePeriodSeconds: 300
      containers:
      - name: feature-worker
        image: registry.example.com/feature-worker:latest
        ports:
        - name: metrics
          containerPort: 9090
        envFrom:
        - configMapRef:
            name: feature-config
        - secretRef:
            name: feature-secrets
        env:
        - name: SERVICE_NAME
          value: "feature-worker"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: WORKER_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        resources:
          requests:
            cpu: "2000m"
            memory: "4Gi"
          limits:
            cpu: "4000m"
            memory: "8Gi"
        volumeMounts:
        - name: workspace
          mountPath: /var/feature/workspaces
        - name: tmp
          mountPath: /tmp
        - name: shm
          mountPath: /dev/shm
        livenessProbe:
          httpGet:
            path: /healthz
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 30
          timeoutSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 9090
          initialDelaySeconds: 10
          periodSeconds: 10
        securityContext:
          runAsNonRoot: true
          runAsUser: 65532
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
      volumes:
      - name: tmp
        emptyDir:
          medium: Memory
          sizeLimit: 1Gi
      - name: shm
        emptyDir:
          medium: Memory
          sizeLimit: 256Mi
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: feature-worker
              topologyKey: kubernetes.io/hostname
  volumeClaimTemplates:
  - metadata:
      name: workspace
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: fast-ssd
      resources:
        requests:
          storage: 50Gi

---
apiVersion: v1
kind: Service
metadata:
  name: feature-worker
  namespace: feature-system
spec:
  clusterIP: None
  selector:
    app: feature-worker
  ports:
  - name: metrics
    port: 9090
    targetPort: 9090
```

### Sandbox Manager DaemonSet

```yaml
# sandbox-manager.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: sandbox-manager
  namespace: feature-system
  labels:
    app: sandbox-manager
spec:
  selector:
    matchLabels:
      app: sandbox-manager
  template:
    metadata:
      labels:
        app: sandbox-manager
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: feature-worker
      hostPID: false
      containers:
      - name: sandbox-manager
        image: registry.example.com/sandbox-manager:latest
        ports:
        - name: grpc
          containerPort: 50051
        - name: metrics
          containerPort: 9090
        envFrom:
        - configMapRef:
            name: feature-config
        env:
        - name: CONTAINERD_SOCKET
          value: "/run/containerd/containerd.sock"
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        resources:
          requests:
            cpu: "1000m"
            memory: "2Gi"
          limits:
            cpu: "2000m"
            memory: "4Gi"
        volumeMounts:
        - name: containerd-socket
          mountPath: /run/containerd/containerd.sock
        - name: sandbox-data
          mountPath: /var/feature/sandboxes
        securityContext:
          privileged: false
          capabilities:
            add:
            - SYS_ADMIN  # Required for container management
            drop:
            - ALL
      volumes:
      - name: containerd-socket
        hostPath:
          path: /run/containerd/containerd.sock
          type: Socket
      - name: sandbox-data
        hostPath:
          path: /var/feature/sandboxes
          type: DirectoryOrCreate
      tolerations:
      - operator: Exists
```

### Horizontal Pod Autoscaler

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: feature-service-hpa
  namespace: feature-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: feature-service
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
      - type: Percent
        value: 100
        periodSeconds: 15
      - type: Pods
        value: 4
        periodSeconds: 15
      selectPolicy: Max

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: llm-orchestrator-hpa
  namespace: feature-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: llm-orchestrator
  minReplicas: 2
  maxReplicas: 8
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60
```

### Pod Disruption Budget

```yaml
# pdb.yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: feature-service-pdb
  namespace: feature-system
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: feature-service

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: feature-worker-pdb
  namespace: feature-system
spec:
  maxUnavailable: 2
  selector:
    matchLabels:
      app: feature-worker
```

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: feature-service-policy
  namespace: feature-system
spec:
  podSelector:
    matchLabels:
      app: feature-service
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: istio-system
    ports:
    - port: 80
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgresql
    ports:
    - port: 5432
  - to:
    - podSelector:
        matchLabels:
          app: redpanda
    ports:
    - port: 9092

---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: feature-worker-policy
  namespace: feature-system
spec:
  podSelector:
    matchLabels:
      app: feature-worker
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: feature-service
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgresql
    ports:
    - port: 5432
  - to:
    - podSelector:
        matchLabels:
          app: redpanda
    ports:
    - port: 9092
  - to:
    - podSelector:
        matchLabels:
          app: git-service
    ports:
    - port: 80
  - to:
    - podSelector:
        matchLabels:
          app: llm-orchestrator
    ports:
    - port: 80
  - to:
    - podSelector:
        matchLabels:
          app: sandbox-manager
    ports:
    - port: 50051
```

---

## Database Migration

### Migration Job

```yaml
# migration-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: feature-migrate
  namespace: feature-system
spec:
  ttlSecondsAfterFinished: 300
  template:
    spec:
      serviceAccountName: feature-service
      restartPolicy: OnFailure
      containers:
      - name: migrate
        image: registry.example.com/feature-service:latest
        command: ["/service-feature"]
        args: ["--migrate"]
        envFrom:
        - configMapRef:
            name: feature-config
        - secretRef:
            name: feature-secrets
        env:
        - name: DO_DATABASE_MIGRATE
          value: "true"
      backoffLimit: 3
```

---

## Ingress Configuration

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: feature-ingress
  namespace: feature-system
  annotations:
    kubernetes.io/ingress.class: nginx
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "300"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "300"
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
  - hosts:
    - feature.api.example.com
    secretName: feature-tls
  rules:
  - host: feature.api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: feature-service
            port:
              number: 80
```

---

## Observability Stack

### ServiceMonitor (Prometheus)

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: feature-services
  namespace: feature-system
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app.kubernetes.io/part-of: feature-platform
  endpoints:
  - port: metrics
    interval: 15s
    path: /metrics
```

### Grafana Dashboards

Deploy pre-built dashboards via ConfigMap:

```yaml
# grafana-dashboards.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feature-dashboards
  namespace: observability
  labels:
    grafana_dashboard: "1"
data:
  feature-overview.json: |
    {
      "title": "Feature Platform Overview",
      "panels": [
        {
          "title": "Feature Executions",
          "type": "stat",
          "targets": [
            {
              "expr": "sum(rate(feature_executions_total[5m]))"
            }
          ]
        }
      ]
    }
```

---

## Deployment Procedures

### Initial Deployment

```bash
# 1. Create namespace
kubectl apply -f namespace.yaml

# 2. Deploy secrets
kubectl apply -f external-secrets.yaml

# 3. Deploy config
kubectl apply -f configmap.yaml

# 4. Run migrations
kubectl apply -f migration-job.yaml
kubectl wait --for=condition=complete job/feature-migrate -n feature-system --timeout=300s

# 5. Deploy services
kubectl apply -f feature-service.yaml
kubectl apply -f feature-worker.yaml
kubectl apply -f git-service.yaml
kubectl apply -f llm-orchestrator.yaml
kubectl apply -f sandbox-manager.yaml

# 6. Deploy networking
kubectl apply -f network-policy.yaml
kubectl apply -f ingress.yaml

# 7. Deploy autoscaling
kubectl apply -f hpa.yaml
kubectl apply -f pdb.yaml

# 8. Verify deployment
kubectl get pods -n feature-system
kubectl get svc -n feature-system
```

### Rolling Update

```bash
# Update image
kubectl set image deployment/feature-service \
  feature-service=registry.example.com/feature-service:v1.2.0 \
  -n feature-system

# Monitor rollout
kubectl rollout status deployment/feature-service -n feature-system

# Rollback if needed
kubectl rollout undo deployment/feature-service -n feature-system
```

### Scaling Workers

```bash
# Scale workers to match partition count
kubectl scale statefulset/feature-worker --replicas=16 -n feature-system

# Verify partition assignment
kubectl logs -l app=feature-worker -n feature-system | grep "partition assigned"
```

---

## Health Checks

### Verify Services

```bash
# Check all pods are running
kubectl get pods -n feature-system -o wide

# Check service endpoints
kubectl get endpoints -n feature-system

# Test API health
curl -k https://feature.api.example.com/healthz

# Check event bus connectivity
kubectl exec -it feature-worker-0 -n feature-system -- \
  /bin/sh -c 'echo "test" | kafkacat -b $EVENT_BUS_BROKERS -P -t test'
```

### Verify Event Processing

```bash
# Check consumer lag
kubectl exec -it redpanda-0 -n feature-system -- \
  rpk group describe feature-workers

# Check worker logs
kubectl logs -f feature-worker-0 -n feature-system
```

---

## Troubleshooting

### Common Issues

| Issue | Symptoms | Resolution |
|-------|----------|------------|
| Worker not processing | High consumer lag | Check worker logs, restart if needed |
| Git clone timeout | Clone failures | Increase timeout, check network |
| LLM rate limiting | 429 errors | Scale orchestrators, implement backoff |
| Sandbox OOM | Containers killed | Increase memory limits |
| Database connection | Connection refused | Check PostgreSQL status |

### Debug Commands

```bash
# Get pod logs
kubectl logs -f deployment/feature-service -n feature-system

# Exec into pod
kubectl exec -it feature-worker-0 -n feature-system -- /bin/sh

# Check events
kubectl get events -n feature-system --sort-by='.lastTimestamp'

# Describe pod
kubectl describe pod feature-worker-0 -n feature-system
```

---

## Backup and Recovery

### Database Backup

```yaml
# backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pg-backup
  namespace: feature-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: postgres:16
            command:
            - /bin/sh
            - -c
            - |
              pg_dump $DATABASE_URL | gzip > /backup/feature-$(date +%Y%m%d).sql.gz
              # Upload to S3
              aws s3 cp /backup/feature-$(date +%Y%m%d).sql.gz s3://backups/feature/
            envFrom:
            - secretRef:
                name: feature-secrets
          restartPolicy: OnFailure
```

### Event Store Backup

Kafka/Redpanda snapshots are managed by the cluster operator. Ensure retention is configured:

```yaml
# Topic configuration
retention.ms: 604800000  # 7 days
retention.bytes: 107374182400  # 100GB per partition
```
