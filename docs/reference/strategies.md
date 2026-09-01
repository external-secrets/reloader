# Strategies

Each destination in the Reloader configuration supports three optional strategies that control how updates are applied, how secrets are matched, and how the Reloader waits between reconciliations.

When not specified, sensible defaults are used per destination type. See [Deployments](../destinations/deployments.md), [ExternalSecrets](../destinations/external-secrets.md), [PushSecrets](../destinations/push-secrets.md), and [WorkflowRunTemplate](../destinations/workflows.md) for their respective defaults.

## Update Strategy

The Update Strategy controls what action the Reloader takes when a matching event arrives.

### Operations

| Operation     | Description                                                                 |
|---------------|-----------------------------------------------------------------------------|
| `PatchStatus` | Patches the status subresource of the destination object.                  |
| `Patch`       | Patches the destination object at a custom path with a given template.      |
| `Delete`      | Deletes the destination object entirely.                                    |

### Example: Custom Patch

```yaml
destinationsToWatch:
  - type: Deployment
    deployment:
      labelSelectors:
        matchLabels:
          app: my-app
    updateStrategy:
      operation: Patch
      patchOperationConfig:
        path: "/spec/template/metadata/annotations"
        template: '{"reloader.external-secrets.io/rotated-at": "{{ .Timestamp }}"}'
```

### Example: Delete on Event

```yaml
destinationsToWatch:
  - type: ExternalSecret
    externalSecret:
      names:
        - my-external-secret
    updateStrategy:
      operation: Delete
```

## Match Strategy

By default, a destination decides for itself whether it's affected by an event: an `ExternalSecret`, for example, is considered affected if the event's identifier exactly matches one of its configured remote keys. A Match Strategy replaces that default check with your own: it evaluates a JSON path on the destination object against one or more conditions, so you can match on a different field, a substring, or a regular expression instead.

A condition's `value` is rendered as a Go template before comparison, with the event bound to the template root, so you can reference the specific event being processed - for example `{{ .SecretIdentifier }}`. Available fields are `SecretIdentifier`, `RotationTimestamp`, `TriggerSource`, and `Namespace`. A `value` with no template actions is used as a literal string.

If `path` resolves to multiple values (for example, a path containing `[*]`), the destination matches if *any* of those values satisfies *all* of the given conditions.

### Condition Operations

| Operation           | Description                                 |
|---------------------|----------------------------------------------|
| `Equal`             | Exact value match.                          |
| `NotEqual`          | Value does not match.                       |
| `Contains`          | Value contains the given substring.         |
| `NotContains`       | Value does not contain the given substring. |
| `ContainedBy`       | Value is a substring of the given value.    |
| `NotContainedBy`    | Value is not a substring of the given value.|
| `RegularExpression` | Value matches the given regular expression. |

### Example: Custom Match Path

```yaml
destinationsToWatch:
  - type: Deployment
    deployment:
      labelSelectors:
        matchLabels: {}
    matchStrategy:
      path: "spec.template.spec.containers[*].env[*].valueFrom.secretKeyRef.name"
      conditions:
        - value: "my-secret"
          operation: Equal
```

### Example: Regex Match

```yaml
destinationsToWatch:
  - type: ExternalSecret
    externalSecret:
      labelSelectors:
        matchLabels: {}
    matchStrategy:
      path: "spec.dataFrom.find.name.regexp"
      conditions:
        - value: "prod/.*"
          operation: RegularExpression
```

### Example: Matching a Friendly Name Against an ARN

Some sources - such as `AwsSqs` reading CloudTrail's `PutSecretValue` events - surface a secret's full ARN as the event identifier, while an `ExternalSecret` conventionally references that same secret by its friendly name. `ContainedBy`, combined with templating, lets the friendly name match against the ARN it's embedded in:

```yaml
destinationsToWatch:
  - type: ExternalSecret
    externalSecret:
      labelSelectors:
        matchLabels: {}
    matchStrategy:
      path: "spec.dataFrom[*].extract.key"
      conditions:
        - value: "{{ .SecretIdentifier }}"
          operation: ContainedBy
```

## Wait Strategy

The Wait Strategy controls how the Reloader waits between reconciling multiple destination objects. This is useful to avoid overwhelming the cluster with simultaneous rollouts.

Two modes are available: time-based and condition-based. They can be used independently or together.

### Time-Based Wait

Waits for a fixed duration before reconciling the next object.

```yaml
destinationsToWatch:
  - type: Deployment
    deployment:
      labelSelectors:
        matchLabels: {}
    waitStrategy:
      time: 30s
```

### Condition-Based Wait

Waits until a specific condition is met on the destination object before moving to the next one. Supports retry logic with configurable timeout and maximum retries.

```yaml
destinationsToWatch:
  - type: Deployment
    deployment:
      labelSelectors:
        matchLabels: {}
    waitStrategy:
      condition:
        type: Available
        status: "True"
        retryTimeout: 10s
        maxRetries: 18
```

In this example, the Reloader waits for the `Available` condition to be `True` before proceeding, checking every 10 seconds up to 18 times (3 minutes total).

### Condition Fields

| Field              | Description                                                              | Required |
|--------------------|--------------------------------------------------------------------------|----------|
| `type`             | The name of the condition to wait for.                                   | Yes      |
| `status`           | The expected status of the condition.                                    | No       |
| `message`          | Optional message to match.                                               | No       |
| `reason`           | Optional reason to match.                                                | No       |
| `retryTimeout`     | Period to wait before each retry.                                        | No       |
| `maxRetries`       | Maximum number of retries for the condition.                             | No       |
| `transitionedAfter`| Only accept this condition after a given period from the transition time. | No       |
| `updatedAfter`     | Only accept this condition after a given period from the update time.     | No       |

### Example: Wait for Fresh Condition

```yaml
waitStrategy:
  condition:
    type: Available
    status: "True"
    transitionedAfter: 5s
    retryTimeout: 10s
    maxRetries: 30
```

This ensures the `Available` condition was set at least 5 seconds ago, avoiding stale conditions from a previous rollout.
