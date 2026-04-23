# Webhook Source

This guide explains how to set up the Webhook notification source for the Reloader component in your environment. Using webhooks as a notification source lets you trigger secret rotation events by sending HTTP POST requests to the Reloader process.

## How it works

The controller runs a **single shared HTTP server** for all `Config` resources. The listen address is set with the controller flag **`--webhook-bind-address`** (default `:8082`). Each cluster-scoped `Config` is exposed at:

`POST /webhook/<Config.metadata.name>`

There is no per-CR URL path or bind address; callers use the `Config` name in the path.

## Configuration

Configure a `NotificationSource` with `type: Webhook` and a `webhook` block. The main field is **`identifierPathOnPayload`** (JSON path in the body where the secret name appears).

### Key fields

* **identifierPathOnPayload**: JSON path in the POST body for the secret identifier. It must match the name of the secret being rotated. If omitted, the default path is `0.data.ObjectName`.
* **webhookAuth** (optional): Basic or bearer authentication for incoming requests.
* **retryPolicy** (optional): Retry failed publishes to the internal event channel.

### Payload structure

The POST body must be JSON containing the secret identifier at the configured path.

#### Example payload

```json
{
  "0": {
    "data": {
      "ObjectName": "my-secret"
    }
  }
}
```

Here the identifier is at `0.data.ObjectName`, matching the secret name `my-secret`.

### Triggering a webhook notification

Send an HTTP POST to the Reloader webhook base URL with path `/webhook/<your-config-name>`.

```bash
curl -X POST "http://<reloader-host>:<webhook-port>/webhook/my-reloader-config" \
  -H "Content-Type: application/json" \
  -d '{
    "0": {
      "data": {
        "ObjectName": "my-secret"
      }
    }
  }'
```

Replace `my-reloader-config` with the `metadata.name` of your `Config` CR.

### Helm

If you use the chart under `deploy/charts/reloader`, set **`service.webhook.enabled: true`**. The chart then adds **`--webhook-bind-address`** and a **`webhook`** container port using **`service.webhook.listenPort`** (default `8090`, aligned with the optional `*-webhook` Service). You can still override the flag with **`extraArgs`** if needed.

There is no default “main” HTTP `Service` on port 8080. **`ingress.enabled`** requires **`service.webhook.enabled`**: the Ingress targets the **`{{ release }}-webhook`** Service on **`service.webhook.port`** (paths such as **`/webhook/...`**).

Any client that can reach the Service or host on that port can trigger rotation as long as the JSON path and optional auth match your `Config`.
