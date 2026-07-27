# Hashicorp Vault

This guide explains how to set up Open Bao or Hashicorp Vault as a notification source for the Reloader component in your environment.

## Overview

The Reloader receives events from Hashicorp Vault via tcp audit log function, and looks for `update` events on a given secret key.

1. **Secrets Stored in Hashicorp Vault**: Your secrets are stored and managed in Hashicorp Vault.
2. **Audit Log Configuration**: Hashicorp Vault is configured to send audit logs via tcp socket to Reloader.
3. **Reloader Listener**: The Reloader includes a tcp listener that receives events from Hashicorp Vault and annotates the corresponding Kubernetes ExternalSecrets to trigger reconciliation and synchronization of the secret values.

## Prerequisites

- A Hashicorp Vault instance installed and available to the cluster.
- Kubernetes cluster with External Secrets Operator and Reloader installed.
- Load Balancer Service Provider available for your Kubernetes cluster.

## Step 1: Configure Reloader

Update your Reloader configuration to set up the Hashicorp Vault listener.

### Configuration Spec

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-vault-sample
  labels:
    app.kubernetes.io/name: reloader
spec:
  notificationSources:
    - type: HashicorpVault
      hashicorpVault:
        host: 0.0.0.0
        port: 8000
  secretsToWatch:
    - labelSelectors:
        matchLabels: {}
```

- **`host`**: The host interface to bind the listener to. Use `0.0.0.0` to listen on all interfaces.
- **`port`**: The port on which the listener will accept connections. Defaults to 8000.

In this configuration:

- The listener will accept events at `tcp://<host>:8000`.

## Step 2: Configure Hashicorp Vault

Ensure you have a Hashicorp Vault set up with the secrets you wish to manage.

```bash
vault kv put -mount secret secret-to-rotate key=value
```

!!! tip
    You need to update your mount path according to your setup.

Next, get the IP Address of the Load Balancer Service for reloader:

```bash
IP_ADDRESS=$(kubectl get service -n reloader-system reloader-controller-manager-socket -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Finally, configure an audit log forwarding rule within your vault:

```bash
vault audit enable socket address=$IP_ADDRESS:8000 socket_type=tcp
```

## Processing Events

When a secret is updated in Hashicorp Vault, an event is sent to the reloader listener via a TCP socket. The Reloader processes the events and annotates the corresponding `ExternalSecrets` to trigger a reconciliation.

In this example - if any changes are made to `secret-to-rotate` in the Vault, the Reloader will process every `ExternalSecret` that contains a reference to `secret-to-rotate`.

## Enabling External Traffic

To ensure the tcp listener is accessible:

- **Network Policies**: If using network policies, allow outbound and inbound traffic from your hashicorp vault instance to the tcp listener.
- **Firewall Rules**: Configure firewalls or security groups to allow traffic from Hashicorp Vault to your cluster.

## Additional Resources

- [Hashicorp Vault Documentation](https://developer.hashicorp.com/vault/api-docs/secret/kv/kv-v2)
- [Audit logs on Hashicorp Vault](https://developer.hashicorp.com/vault/docs/commands/audit/enable)
- [External Secrets Operator Documentation](https://external-secrets.io/)
