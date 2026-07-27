# Webhook Source

This guide explains how to set up the Webhook notification source for the Reloader component in your environment. Using Webhooks as a notification source allows you to trigger secret rotation events via HTTP calls to your Webhook endpoint.

## Configuration

To configure a Webhook as a notification source, the Reloader needs to be provided with the URL path to listen on, as well as the identifier in the payload that refers to the secret being rotated.

### Key Fields

* **path**: Specifies the Webhook path that the Reloader will listen to. This is the endpoint where Webhook notifications will be received.
* **identifierPathOnPayload**: Defines the key in the payload that contains the secret identifier. The identifier must match the name of the secret being rotated. By default, the path is `0.data.ObjectName`.

### Payload Structure

The Webhook notification must contain a payload with a secret identifier. The Reloader will extract this identifier based on the path defined in the configuration.

#### Example Payload

```json
{
  "0": {
    "data": {
      "ObjectName": "my-secret"
    }
  }
}
```

In this example, the Webhook payload contains a secret identifier at `0.data.ObjectName`, which corresponds to the secret named `my-secret`. The Reloader will use this identifier to rotate the appropriate secret.

### Triggering a webhook notification

To trigger a secret rotation, send an HTTP POST request to the Webhook endpoint you've configured.

```bash
curl -X POST https://your-rotator-endpoint/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "0": {
      "data": {
        "ObjectName": "my-secret"
      }
    }
  }'
```

Once this request is received by the Reloader, it will extract the secret identifier and proceed with the rotation process for the specified secret.

Any service that can call an endpoint can trigger the rotation as long as you configure the keys accordingly.
