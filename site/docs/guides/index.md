# Guides

Practical how-to guides for working with SpecGraph. Each guide is self-contained —
jump to whichever workflow you need.

<div class="grid cards" markdown>

- :material-console: **CLI Cookbook**

    ---

    Step-by-step recipes for common CLI workflows — authoring, graph queries,
    slices, drift, linting, and execution lifecycle.

    [:octicons-arrow-right-24: CLI Cookbook](cli-cookbook.md)

- :material-sync: **Sync & Integration**

    ---

    Push specs to Beads and GitHub Issues for external visibility and
    coordination.

    [:octicons-arrow-right-24: Sync & Integration](sync.md)

- :material-login: **Interactive OIDC Login**

    ---

    Enable browser-based "Sign in with &lt;provider&gt;" login (Authorization
    Code + PKCE → opaque server session) with Microsoft Entra ID, Okta, and
    GitHub-via-broker configuration examples.

    [:octicons-arrow-right-24: Interactive OIDC Login](oidc-login.md)

- :material-heart-pulse: **Health Probes (Kubernetes / Knative)**

    ---

    Enable plain-HTTP `/livez` and `/readyz` endpoints on a dedicated
    listener for kubelet `httpGet` probes, with Knative and K8s YAML
    examples.

    [:octicons-arrow-right-24: Health Probes](health-probes.md)

- :material-chart-line: **Observability (OpenTelemetry)**

    ---

    Enable OpenTelemetry traces, metrics, and logs for the server and CLI —
    master switch, sampling, context-aware logging, and the emitted signal
    inventory.

    [:octicons-arrow-right-24: Observability](observability.md)

</div>
