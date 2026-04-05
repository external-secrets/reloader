# Azure Event Grid

This guide explains how to set up the Azure Event Grid notification source for the Reloader component in your environment. Using Azure Event Grid allows you to trigger secret rotation events based on updates made to your secrets in Azure Key Vault.

## Overview

The Reloader listens to Azure Event Grid events to determine when to trigger secret rotations. The overall process is as follows:

1. **Secrets Stored in Azure Key Vault**: Your secrets are stored and managed in Azure Key Vault.
2. **Event Grid Configuration**: Azure Event Grid is configured to publish events related to secret changes to a webhook endpoint.
3. **Reloader Listener**: The Reloader includes a webhook listener that receives events from Event Grid and annotates the corresponding Kubernetes ExternalSecrets to trigger reconciliation and synchronization of the secret values.

## Prerequisites

- An Azure account with permissions to manage Key Vault, Event Grid, and create subscriptions.
- Kubernetes cluster with External Secrets Operator and Reloader installed.
- Access to create Kubernetes Services and Ingress resources.
- Domain name and TLS certificates if using HTTPS (required by Azure Event Grid).

!!! important
    Azure Event Grid requires webhook endpoints to be HTTPS with a valid, trusted certificate. Self-signed certificates or IP addresses are not supported. Ensure you have a domain name and valid TLS certificate for your webhook endpoint.

## Step 1: Configure Azure Key Vault

Ensure you have an Azure Key Vault set up with the secrets you wish to manage.

1. Sign in to the [Azure Portal](https://portal.azure.com/).
2. Navigate to **Key Vaults** and select your Key Vault or create a new one.
3. Add or update secrets as needed.

## Step 2: Configure Reloader

Update your Reloader configuration to set up the Event Grid listener.

### Event Grid Listener Configuration

The Reloader needs to run a webhook listener to receive events from Azure Event Grid. The configuration for the listener is specified using the `AzureEventGrid` notification source type.

### Configuration Spec

```yaml
type: AzureEventGrid
azureEventGrid:
  host: string
  port: int32
  subscriptions:
    - string
```

- **`host`**: The host interface to bind the listener to. Use `0.0.0.0` to listen on all interfaces.
- **`port`**: The port on which the listener will accept connections.
- **`subscriptions`**: A list of subscription names. For each subscription, the listener will create a unique path to listen to events specific to that subscription.

### Example Configuration

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-azure-sample
spec:
  notificationSources:
    - type: AzureEventGrid
      azureEventGrid:
        host: 0.0.0.0
        port: 8080
        subscriptions:
          - my-event-subscription
  destinationsToWatch:
    - type: ExternalSecret
      externalSecret:
        labelSelectors:
          matchLabels:
            app: my-app
```

In this configuration:

- The listener will accept events at `http://<host>:8080/my-event-subscription`.
- You can specify multiple subscriptions if needed.

## Step 3: Expose the Webhook Listener

To allow Azure Event Grid to send events to the webhook listener, you need to expose it outside the Kubernetes cluster.

### Create a Kubernetes Service

Create a Service to expose the Reloader webhook listener.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: reloader-webhook
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: reloader
  ports:
    - protocol: TCP
      port: 8080
      targetPort: 8080
```

### Configure Ingress

Use an Ingress resource to expose the service externally over HTTPS.

#### Example Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: reloader-azure-webhook-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - your.domain.com
      secretName: reloader-azure-webhook-tls
  rules:
    - host: your.domain.com
      http:
        paths:
          - path: /my-event-subscription
            pathType: Prefix
            backend:
              service:
                name: reloader-webhook
                port:
                  number: 8080
```

- Replace `your.domain.com` with your actual domain.
- Ensure that `cert-manager` is installed and configured to obtain TLS certificates.
- Update annotations based on your Ingress controller and TLS setup.

### DNS Configuration

- Create a DNS `A` record pointing `your.domain.com` to the external IP address of your Ingress controller.

### Ensure HTTPS Access

- Verify that `https://your.domain.com/my-event-subscription` is accessible externally.
- Use tools like `curl` or a web browser to test the endpoint.

## Step 4: Configure Azure Event Grid Subscription

Set up Azure Event Grid to send events when secrets in Key Vault are updated.

### Create an Event Grid Subscription

1. Navigate to your Key Vault in the Azure Portal.
2. Under **Events**, click on **+ Event Subscription**.
3. Fill in the **Name** for your subscription (e.g., `my-event-subscription`).
4. In **Event Schema**, select **Event Grid Schema**.
5. Under **Event Types**, select the events you want to subscribe to:
    - **Microsoft.KeyVault.SecretNewVersionCreated**
    - **Microsoft.KeyVault.SecretNearExpiry** (optional)
6. In **Endpoint Type**, select **Web Hook**.
7. For the **Endpoint URL**, enter `https://your.domain.com/my-event-subscription`.
8. Optionally, configure additional settings such as filters.
9. Click **Create** to save the subscription.

!!! note
    Azure Event Grid will perform a validation handshake by sending a validation event to the webhook endpoint. The Reloader listener must be running and accessible at the specified URL for validation to succeed.

### Validation Handshake

- The Reloader's webhook listener handles the validation automatically.
- If validation fails, ensure:
    - The listener is running.
    - The Ingress and Service are correctly configured.
    - The TLS certificate is valid and trusted.
    - There are no firewall or network policies blocking traffic.

## Step 5: Deploy the Reloader

Apply the Reloader configuration to your Kubernetes cluster:

```bash
kubectl apply -f reloader-azure-sample.yaml
kubectl apply -f reloader-webhook-service.yaml
kubectl apply -f reloader-azure-webhook-ingress.yaml
```

Replace the filenames with the actual filenames of your configuration files.

## Processing Event Grid Events

When a secret is updated in Azure Key Vault, an event is sent to the webhook listener via Event Grid. The Reloader processes the events and annotates the corresponding Kubernetes ExternalSecrets to trigger a reconciliation.

### Azure Event Grid Event Schema

```json
[
  {
    "id": "12345-abcde-67890-fghij",
    "eventType": "Microsoft.KeyVault.SecretNewVersionCreated",
    "subject": "/subscriptions/{subscription-id}/resourceGroups/{resource-group}/providers/Microsoft.KeyVault/vaults/{vault-name}",
    "eventTime": "2024-11-02T12:00:00Z",
    "data": {
      "Id": "https://{vault-name}.vault.azure.net/secrets/{secret-name}/{version-id}",
      "VaultName": "{vault-name}",
      "ObjectType": "Secret",
      "Version": "{version-id}",
      "NBF": null,
      "EXP": null
    },
    "dataVersion": "1",
    "metadataVersion": "1"
  }
]
```

The Reloader looks for the `eventType` and `data.Id` fields in the event payload to identify which secret has changed.

## Additional Configuration Options

- **Multiple Subscriptions**: You can specify multiple subscriptions in the `subscriptions` list. Each subscription will have a unique path.

    ```yaml
    subscriptions:
      - subscription-one
      - subscription-two
    ```

- **Host and Port**: Adjust the `host` and `port` as needed based on your deployment environment.
- **Ingress Annotations**: Depending on your Ingress controller (e.g., NGINX, Traefik), you may need to add specific annotations to enable features like TLS termination or client IP preservation.

## Enabling External Traffic

To ensure the webhook listener is accessible:

- **Network Policies**: If using network policies, allow inbound traffic to the webhook listener.
- **Firewall Rules**: Configure firewalls or security groups to allow traffic from Azure Event Grid to your cluster.
- **Ingress Controller**: Make sure your Ingress controller is properly set up to handle external traffic.

### Using Ingress Controllers

Ingress controllers manage external access to services in a Kubernetes cluster.

- **Install an Ingress Controller**: Choose an Ingress controller compatible with your environment.
- **Configure TLS**: Use `cert-manager` to automatically obtain and renew TLS certificates from Let's Encrypt.

    ```yaml
    apiVersion: cert-manager.io/v1
    kind: ClusterIssuer
    metadata:
      name: letsencrypt-prod
    spec:
      acme:
        server: https://acme-v02.api.letsencrypt.org/directory
        email: your-email@example.com
        privateKeySecretRef:
          name: letsencrypt-prod
        solvers:
          - http01:
              ingress:
                class: nginx
    ```

- **Update DNS Records**: Point your domain to the Ingress controller's external IP.

## Troubleshooting

### Event Grid Subscription Validation Fails

- **Check Listener Accessibility**: Ensure the webhook endpoint is accessible over HTTPS from the internet.
- **Verify TLS Certificate**: The TLS certificate must be valid and signed by a trusted Certificate Authority.
- **Inspect Logs**: Check the Reloader pod logs for any errors during validation.

### Events Not Being Processed

- **Verify Event Subscription**: Ensure the Event Grid subscription is active and events are being sent.
- **Confirm Secret IDs**: The `data.Id` in the event must match the secret IDs in your Kubernetes ExternalSecrets.
- **Review Network Policies**: Ensure network policies are not blocking incoming events.

## Additional Resources

- [Azure Key Vault Documentation](https://docs.microsoft.com/azure/key-vault/)
- [Azure Event Grid Documentation](https://docs.microsoft.com/azure/event-grid/)
- [Azure Event Grid Webhook Delivery](https://docs.microsoft.com/azure/event-grid/delivery-and-retry)
- [External Secrets Operator Documentation](https://external-secrets.io/)
