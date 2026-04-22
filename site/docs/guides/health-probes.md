# Health Probes for Kubernetes and Knative

SpecGraph can expose plain-HTTP liveness and readiness endpoints on a
dedicated listener so `kubelet`'s `httpGet` probes work without needing to
wrap ConnectRPC framing.

## Why a Separate Listener

The main API speaks ConnectRPC — `POST /specgraph.v1.ServerService/Health`
expects `Content-Type: application/json` and `Connect-Protocol-Version: 1`
headers plus a `{}` body. A plain `GET /` on the main port only hits the
embedded web UI, which returns 200 even when the DB is offline — not a
real health signal.

A second listener with `GET /livez` and `GET /readyz` gives you:

- **Plain-HTTP probes** that kubelet can issue without custom headers
- **Port isolation** — firewall / NetworkPolicy the main API while leaving
  probes reachable to the kubelet
- **Cheap liveness** that never touches Postgres
- **Cached readiness** — a background goroutine probes the DB every 5s,
  so kubelet polling doesn't generate DB load that scales with replica
  count

## Enable the Probes Listener

Probes are **off by default**. Set `server.probes.listen` in
`~/.config/specgraph/config.yaml` (or the file mounted at your container's
`--config` path) to enable them:

```yaml
server:
  listen: "0.0.0.0:9090"
  probes:
    listen: "0.0.0.0:9091"
```

- `GET /livez` — 200 as long as the HTTP goroutine is alive. No DB touch.
- `GET /readyz` — 200 when the last cached Postgres probe succeeded;
  503 otherwise (including before the first probe completes).

The probe server shuts down with the main server on SIGINT/SIGTERM.

## Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: specgraph
spec:
  template:
    spec:
      containers:
        - name: specgraph
          image: ghcr.io/specgraph/specgraph:latest
          args: ["serve", "--config", "/etc/specgraph/config.yaml"]
          ports:
            - name: api
              containerPort: 9090
            - name: probes
              containerPort: 9091
          livenessProbe:
            httpGet:
              path: /livez
              port: probes
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: probes
            initialDelaySeconds: 2
            periodSeconds: 5
            failureThreshold: 2
          volumeMounts:
            - name: config
              mountPath: /etc/specgraph
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: specgraph-config
```

## Knative Service Example

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: specgraph
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/specgraph/specgraph:latest
          args: ["serve", "--config", "/etc/specgraph/config.yaml"]
          ports:
            - containerPort: 9090
          readinessProbe:
            httpGet:
              path: /readyz
              port: 9091
          livenessProbe:
            httpGet:
              path: /livez
              port: 9091
```

Knative's activator proxies traffic to the containerPort (9090); the probe
port (9091) is dialed directly by the kubelet against the pod IP.

## Tuning Notes

- The readiness cache refreshes every **5 seconds** by default with a
  **2-second** per-probe timeout. Override via YAML when your workload
  needs different timing:

  ```yaml
  server:
    probes:
      listen: "0.0.0.0:9091"
      interval: 10s        # how often to ping the DB (default 5s)
      probe_timeout: 3s    # per-ping deadline (default 2s)
  ```

  Both durations use Go's `time.Duration` syntax (`500ms`, `2s`, `1m`).
  `probe_timeout` must not exceed `interval` — otherwise probes would
  overlap and stack up behind a slow Postgres; the server rejects such
  configs at startup.
- The first probe runs at startup, not after the first tick, so
  `initialDelaySeconds` can be as low as 2s for readiness without racing
  against "not yet probed" 503s.
- `/readyz` returning 503 does **not** restart the pod — it just stops
  traffic routing. `/livez` 5xx **does** restart. Keep liveness cheap.
