# TCP Socket

This guide explains how to set up a generic TCP Socket as a notification source for the Reloader component in your environment. Using a TCP Socket allows you to trigger secret rotation events from any system capable of sending data over a TCP connection.

## Overview

The Reloader opens a TCP listener that receives JSON payloads from external systems. When a payload arrives, the Reloader extracts the secret identifier from a configurable path in the payload and triggers reconciliation on matching destinations.

This is the same mechanism used internally by the [Hashicorp Vault](vault.md) source, exposed as a standalone source for custom integrations.

## Prerequisites

- Kubernetes cluster with Reloader installed.
- Load Balancer Service Provider available for your Kubernetes cluster (if the sender is external).

## Step 1: Configure Reloader

Update your Reloader configuration to set up the TCP Socket listener.

### Configuration Spec

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-tcp-sample
  labels:
    app.kubernetes.io/name: reloader
spec:
  notificationSources:
    - type: TCPSocket
      tcpSocket:
        host: 0.0.0.0
        port: 8000
        identifierPathOnPayload: "0.data.ObjectName"
  destinationsToWatch:
    - type: ExternalSecret
      externalSecret:
        labelSelectors:
          matchLabels: {}
```

- **`host`**: The host interface to bind the listener to. Use `0.0.0.0` to listen on all interfaces.
- **`port`**: The port on which the listener will accept connections. Defaults to `8000`.
- **`identifierPathOnPayload`**: The key in the payload used to identify the secret. Defaults to `0.data.ObjectName` if not specified.

In this configuration:

- The listener will accept events at `tcp://<host>:8000`.

## Step 2: Expose the TCP Listener

To allow external systems to send events to the TCP listener, you need to expose it outside the Kubernetes cluster.

### Create a Kubernetes Service

Create a LoadBalancer Service to expose the Reloader TCP listener.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: reloader-tcp-socket
spec:
  type: LoadBalancer
  selector:
    app.kubernetes.io/name: reloader
  ports:
    - protocol: TCP
      port: 8000
      targetPort: 8000
```

## Processing Events

When a message arrives on the TCP socket, the Reloader extracts the secret identifier from the payload using the configured `identifierPathOnPayload` path. It then matches the identifier against the configured destinations and triggers reconciliation on any matching resources.

### Example Payload

The default identifier path expects a payload structured like:

```json
{
  "0": {
    "data": {
      "ObjectName": "my-secret"
    }
  }
}
```

You can customize `identifierPathOnPayload` to match the structure of your system's payload.

## Enabling External Traffic

To ensure the TCP listener is accessible:

- **Network Policies**: If using network policies, allow inbound traffic to the TCP listener.
- **Firewall Rules**: Configure firewalls or security groups to allow traffic from the sending system to your cluster.
