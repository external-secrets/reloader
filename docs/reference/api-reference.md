# API Reference

## Kubernetes CRDs

### `reloader.external-secrets.io/v1alpha1`

Package `v1alpha1` contains API Schema definitions for the Reloader `v1alpha1` API group.

#### `Config`

Config is the Schema for the Reloader Config API.

| Field       | Type   | Description                                                     | Validation |
|------------|--------|-----------------------------------------------------------------|------------|
| `apiVersion` | string | `reloader.external-secrets.io/v1alpha1`                         |            |
| `kind`       | string | `Config`                                                       |            |
| `metadata`   | [ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta) | Refer to Kubernetes API documentation for fields of `metadata`. |            |
| `spec`       | [ConfigSpec](#configspec) |  |            |

## Types

### `AWSSDKAuth`

AWSSDKAuth contains authentication methods for AWS SDK.

**Used by:** [AWSSQSConfig](#awssqsconfig)

| Field              | Type                                  | Description | Validation |
|-------------------|---------------------------------------|-------------|------------|
| `authMethod`       | string                                |             |            |
| `region`           | string                                |             |            |
| `serviceAccountRef`| [ServiceAccountSelector](#serviceaccountselector) |             |            |
| `secretRef`        | [AWSSDKSecretRef](#awssdksecretref)   |             |            |

### `AWSSDKSecretRef`

**Used by:** [AWSSDKAuth](#awssdkauth)

| Field                    | Type                                  | Description | Validation |
|--------------------------|---------------------------------------|-------------|------------|
| `accessKeyIdSecretRef`   | [SecretKeySelector](#secretkeyselector) |             |            |
| `secretAccessKeySecretRef` | [SecretKeySelector](#secretkeyselector) |             |            |

### `AWSSQSConfig`

AWSSQSConfig contains configuration for AWS SDK.

**Used by:** [NotificationSource](#notificationsource)

| Field               | Type                        | Description                                                                 | Validation      |
|---------------------|-----------------------------|-----------------------------------------------------------------------------|-----------------|
| `queueURL`          | string                      | QueueURL is the URL of the AWS SDK queue.                                   |                 |
| `auth`              | [AWSSDKAuth](#awssdkauth)   | Authentication methods for AWS.                                             |                 |
| `numberOfMessages`  | integer                     | MaxNumberOfMessages specifies the maximum number of messages to retrieve from the SDK queue in a single request. | `default: 10`   |
| `waitTimeSeconds`   | integer                     | WaitTimeSeconds specifies the duration (in seconds) to wait for messages in the SDK queue before returning. | `default: 20`   |
| `visibilityTimeout` | integer                     | VisibilityTimeout specifies the duration (in seconds) that a message received from the SDK queue is hidden from subsequent retrievals. | `default: 30`   |

### `AzureEventGridConfig`

**Used by:** [NotificationSource](#notificationsource)

| Field           | Type           | Description | Validation        |
|----------------|----------------|-------------|-------------------|
| `host`         | string         |             |                   |
| `port`         | integer        |             | `default: 8080`   |
| `subscriptions`| string array   |             |                   |

### `BasicAuth`

BasicAuth contains basic authentication credentials.

**Used by:** [WebhookAuth](#webhookauth)

| Field               | Type                                  | Description                                               | Validation |
|--------------------|---------------------------------------|-----------------------------------------------------------|------------|
| `usernameSecretRef`| [SecretKeySelector](#secretkeyselector) | UsernameSecretRef contains a secret reference for the username |            |
| `passwordSecretRef`| [SecretKeySelector](#secretkeyselector) | PasswordSecretRef contains a secret reference for the password |            |

### `BearerToken`

BearerToken contains the bearer token credentials.

**Used by:** [WebhookAuth](#webhookauth)

| Field                  | Type                                  | Description                                                                 | Validation |
|------------------------|---------------------------------------|-----------------------------------------------------------------------------|------------|
| `bearerTokenSecretRef` | [SecretKeySelector](#secretkeyselector) | BearerTokenSecretRef references a Kubernetes Secret containing the bearer token. |            |

### `Condition`

**Used by:** [MatchStrategy](#matchstrategy)

| Field       | Type                                | Description | Validation |
|-------------|-------------------------------------|-------------|------------|
| `value`     | string                              |             |            |
| `operation` | [ConditionOperation](#conditionoperation) |             |            |

### `ConditionOperation` _(string)_

**Used by:** [Condition](#condition)

| Field              | Description |
|-------------------|-------------|
| `Equal`           |             |
| `NotEqual`        |             |
| `Contains`        |             |
| `NotContains`     |             |
| `RegularExpression` |          |

### `ConfigSpec`

ConfigSpec defines the desired state of a Reloader Config.

**Used by:** [Config](#config)

| Field                 | Type                                                 | Description                                                                 | Validation |
|----------------------|------------------------------------------------------|-----------------------------------------------------------------------------|------------|
| `notificationSources`| [NotificationSource](#notificationsource) array      | NotificationSources specifies the notification systems to listen to.       |            |
| `destinationsToWatch`| [DestinationToWatch](#destinationtowatch) array      | DestinationsToWatch specifies which secrets the controller should monitor. |            |

### `DeploymentDestination`

Defines a DeploymentDestination. Behavior is a pod template annotations patch.

- Default **UpdateStrategy** is pod template annotations patch to trigger a new rollout.
- Default **MatchStrategy** matches secret keys using:
    - `spec.template.spec.containers[*].env[*].valueFrom.secretKeyRef.name`
    - `spec.template.spec.containers[*].envFrom.secretRef.name`
- Default **WaitStrategy** waits for rollout completion with a 3-minute grace period.

**Used by:** [DestinationToWatch](#destinationtowatch)

| Field               | Type                                                                 | Description                                                                                                              | Validation |
|--------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------|------------|
| `namespaceSelectors`| [LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta) array | NamespaceSelectors selects namespaces based on labels. Manifest must be in a matching namespace.                        |            |
| `labelSelectors`    | [LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)       | LabelSelectors selects resources by labels. Supports `matchLabels` and `matchExpressions`.                              |            |
| `names`             | string array                                                         | Names specifies resource names to watch. The resource must match one of the entries.                                    |            |

### `DestinationToWatch`

DestinationToWatch specifies the criteria for monitoring secrets in the cluster.

**Used by:** [ConfigSpec](#configspec)

| Field            | Type                                                      | Description                                                                                         | Validation |
|------------------|-----------------------------------------------------------|-----------------------------------------------------------------------------------------------------|------------|
| `type`           | enum[`ExternalSecret`, `Deployment`]                      | Type specifies the kind of destination to watch.                                                    |            |
| `externalSecret` | [ExternalSecretDestination](#externalsecretdestination)   |                                                                                                     |            |
| `deployment`     | [DeploymentDestination](#deploymentdestination)           |                                                                                                     |            |
| `updateStrategy` | [UpdateStrategy](#updatestrategy)                         | If not specified, the default update strategy is used.                                              |            |
| `matchStrategy`  | [MatchStrategy](#matchstrategy)                           | If not specified, the default match strategy is used.                                               |            |
| `waitStrategy`   | [WaitStrategy](#waitstrategy)                             | If not specified, the default wait strategy is used.                                                |            |

### `ExternalSecretDestination`

Defines an ExternalSecretDestination. Behavior is annotations patch.

- Default **UpdateStrategy**: annotations patch triggers externalSecret reconcile.
- Default **MatchStrategy**:
    - `spec.data.remoteRef.key`
    - `spec.dataFrom.remoteRef.key`
    - Regex match for `spec.dataFrom.find.name.regexp`

**Used by:** [DestinationToWatch](#destinationtowatch)

| Field               | Type                                                                 | Description                                                                                                              | Validation |
|--------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------|------------|
| `namespaceSelectors`| [LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta) array | NamespaceSelectors selects namespaces based on labels. Manifest must be in a matching namespace.                        |            |
| `labelSelectors`    | [LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#labelselector-v1-meta)       | LabelSelectors selects resources by labels. Supports `matchLabels` and `matchExpressions`.                              |            |
| `names`             | string array                                                         | Names specifies resource names to watch. The resource must match one of the entries.                                    |            |

### `GCPSMAuthSecretRef`

**Used by:** [GooglePubSubAuth](#googlepubsubauth)

| Field                     | Type                                  | Description                             | Validation |
|---------------------------|---------------------------------------|-----------------------------------------|------------|
| `secretAccessKeySecretRef`| [SecretKeySelector](#secretkeyselector) | The SecretAccessKey is used for authentication |            |

### `GCPWorkloadIdentity`

**Used by:** [GooglePubSubAuth](#googlepubsubauth)

| Field              | Type                                | Description | Validation |
|-------------------|-------------------------------------|-------------|------------|
| `serviceAccountRef`| [ServiceAccountSelector](#serviceaccountselector) |             |            |
| `clusterLocation` | string                              |             |            |
| `clusterName`     | string                              |             |            |
| `clusterProjectID`| string                              |             |            |

### `GooglePubSubAuth`

GooglePubSubAuth contains authentication methods for Google Pub/Sub.

**Used by:** [GooglePubSubConfig](#googlepubsubconfig)

| Field              | Type                                        | Description                             | Validation |
|-------------------|---------------------------------------------|-----------------------------------------|------------|
| `secretRef`        | [GCPSMAuthSecretRef](#gcpsmauthsecretref)   |                                         |            |
| `workloadIdentity` | [GCPWorkloadIdentity](#gcpworkloadidentity) |                                         |            |

### `GooglePubSubConfig`

GooglePubSubConfig contains configuration for Google Pub/Sub.

**Used by:** [NotificationSource](#notificationsource)

| Field           | Type                                 | Description                                                     | Validation |
|----------------|--------------------------------------|-----------------------------------------------------------------|------------|
| `subscriptionID`| string                               | SubscriptionID is the ID of the Pub/Sub subscription.           |            |
| `projectID`     | string                               | ProjectID is the GCP project ID where the subscription exists.  |            |
| `auth`          | [GooglePubSubAuth](#googlepubsubauth)| Authentication methods for Google Pub/Sub.                      |            |

### `HashicorpVaultConfig`

HashicorpVault contains configuration for Hashicorp Vault notifications.

**Used by:** [NotificationSource](#notificationsource)

| Field   | Type    | Description                                    | Validation      |
|---------|---------|------------------------------------------------|-----------------|
| `host`  | string  | Host is the hostname or IP address to listen on. |                 |
| `port`  | integer | Port is the port number to listen on.          | `default: 8000` |

### `KubeConfigRef`

**Used by:** [KubernetesAuth](#kubernetesauth)

| Field       | Type                                  | Description | Validation |
|-------------|---------------------------------------|-------------|------------|
| `secretRef` | [SecretKeySelector](#secretkeyselector) |             |            |

### `KubernetesAuth`

**Used by:** [KubernetesSecretConfig](#kubernetessecretconfig)

| Field              | Type                                        | Description                                                    | Validation |
|-------------------|---------------------------------------------|----------------------------------------------------------------|------------|
| `kubeConfigRef`     | [KubeConfigRef](#kubeconfigref)              |                                                                |            |
| `caBundle`          | string                                       | Defines a CABundle if either `tokenRef` or `serviceAccountRef` are used. |            |
| `tokenRef`          | [TokenRef](#tokenref)                        |                                                                |            |
| `serviceAccountRef` | [ServiceAccountSelector](#serviceaccountselector) |                                                                |            |

### `KubernetesSecretConfig`

KubernetesSecretConfig contains configuration for Kubernetes notifications.

**Used by:** [NotificationSource](#notificationsource)

| Field       | Type                                 | Description                                                                 | Validation |
|-------------|--------------------------------------|-----------------------------------------------------------------------------|------------|
| `serverURL` | string                               | Server URL                                                                  |            |
| `auth`      | [KubernetesAuth](#kubernetesauth)    | How to authenticate with Kubernetes. If not specified, default config is used. |            |

### `MatchStrategy`

**Used by:** [DestinationToWatch](#destinationtowatch)

| Field        | Type                            | Description | Validation |
|--------------|----------------------------------|-------------|------------|
| `path`       | string                          |             |            |
| `conditions` | [Condition](#condition) array   |             |            |

### `MockConfig`

MockConfig represents configuration settings for mock notifications.

**Used by:** [NotificationSource](#notificationsource)

| Field          | Type    | Description | Validation |
|----------------|---------|-------------|------------|
| `emitInterval` | integer |             |            |

### `NotificationSource`

NotificationSource represents a notification system configuration.

**Used by:** [ConfigSpec](#configspec)

| Field             | Type                                                  | Description                                                                 | Validation |
|------------------|-------------------------------------------------------|-----------------------------------------------------------------------------|------------|
| `type`           | enum[`AwsSqs`, `AzureEventGrid`, `GooglePubSub`, `HashicorpVault`, `Webhook`, `TCPSocket`, `KubernetesSecret`] | Type of the notification source.                                           |            |
| `awsSqs`         | [AWSSQSConfig](#awssqsconfig)                         | AwsSqs configuration (required if `type` is `AwsSqs`).                      |            |
| `azureEventGrid` | [AzureEventGridConfig](#azureeventgridconfig)         |                                                                             |            |
| `googlePubSub`   | [GooglePubSubConfig](#googlepubsubconfig)             | GooglePubSub configuration (required if `type` is `GooglePubSub`).         |            |
| `webhook`        | [WebhookConfig](#webhookconfig)                       | Webhook configuration (required if `type` is `Webhook`).                   |            |
| `hashicorpVault` | [HashicorpVaultConfig](#hashicorpvaultconfig)         | HashicorpVault configuration (required if `type` is `HashicorpVault`).     |            |
| `kubernetesSecret`| [KubernetesSecretConfig](#kubernetessecretconfig)    | Kubernetes Secret configuration (required if `type` is `KubernetesSecret`).|            |
| `tcpSocket`      | [TCPSocketConfig](#tcpsocketconfig)                   | TCPSocket configuration (required if `type` is `TCPSocket`).               |            |
| `mock`           | [MockConfig](#mockconfig)                             | Mock configuration (optional; useful for testing).                          |            |

### `PatchOperationConfig`

**Used by:** [UpdateStrategy](#updatestrategy)

| Field       | Type    | Description | Validation |
|-------------|---------|-------------|------------|
| `path`      | string  |             |            |
| `template`  | string  |             |            |

### `RetryPolicy`

**Used by:** [WebhookConfig](#webhookconfig)

| Field        | Type    | Description                                                                                                   | Validation |
|--------------|---------|---------------------------------------------------------------------------------------------------------------|------------|
| `maxRetries` | integer | MaxRetries is the maximum number of times to retry. Values over 10 are capped at 10.                         |            |
| `algorithm`  | string  | Defines how retry timing evolves. Supports `"linear"` and `"exponential"` (default if value is invalid/null). |            |

### `SecretKeySelector`

SecretKeySelector references a specific key within a Kubernetes secret.

**Used by:** [AWSSDKSecretRef](#awssdksecretref), [BasicAuth](#basicauth), [BearerToken](#bearertoken), [GCPSMAuthSecretRef](#gcpsmauthsecretref), [KubeConfigRef](#kubeconfigref), [TokenRef](#tokenref)

| Field       | Type    | Description                                                                    | Validation |
|-------------|---------|--------------------------------------------------------------------------------|------------|
| `name`      | string  | Name of the referenced Kubernetes secret.                                      |            |
| `key`       | string  | Key within the referenced Kubernetes secret.                                   |            |
| `namespace` | string  | Namespace where the secret resides.                                            |            |

### `ServiceAccountSelector`

**Used by:** [AWSSDKAuth](#awssdkauth), [GCPWorkloadIdentity](#gcpworkloadidentity), [KubernetesAuth](#kubernetesauth)

| Field       | Type          | Description                                                                                         | Validation |
|-------------|---------------|-----------------------------------------------------------------------------------------------------|------------|
| `name`      | string         | Name of the service account to select.                                                              |            |
| `namespace` | string         | Namespace of the service account.                                                                  |            |
| `audiences` | string array   | Audiences for the service account token. Additional values added based on identity provider used. |            |

### `TCPSocketConfig`

TCPSocketConfig contains configuration for TCP Socket notifications.

**Used by:** [NotificationSource](#notificationsource)

| Field                   | Type    | Description                                                                                                          | Validation        |
|------------------------|---------|----------------------------------------------------------------------------------------------------------------------|-------------------|
| `host`                 | string  | Host is the hostname or IP address to listen on.                                                                     |                   |
| `port`                 | integer | Port is the port number to listen on.                                                                                | `default: 8000`   |
| `identifierPathOnPayload` | string | Key in the payload used to identify the secret. Defaults to `0.data.ObjectName` if not specified.                  |                   |

### `TokenRef`

**Used by:** [KubernetesAuth](#kubernetesauth)

| Field       | Type                                  | Description | Validation |
|-------------|---------------------------------------|-------------|------------|
| `secretRef` | [SecretKeySelector](#secretkeyselector) |             |            |

### `UpdateStrategy`

**Used by:** [DestinationToWatch](#destinationtowatch)

| Field                | Type                                           | Description                          | Validation |
|---------------------|------------------------------------------------|--------------------------------------|------------|
| `operation`          | [UpdateStrategyOperation](#updatestrategyoperation) |                                      |            |
| `patchOperationConfig` | [PatchOperationConfig](#patchoperationconfig)     | Required if `operation == Patch`.    |            |

### `UpdateStrategyOperation` _(string)_

**Used by:** [UpdateStrategy](#updatestrategy)

| Field         | Description |
|---------------|-------------|
| `PatchStatus` |             |
| `Patch`       |             |
| `Delete`      |             |

### `WaitForCondition`

**Used by:** [WaitStrategy](#waitstrategy)

| Field               | Type        | Description                                                                                      | Validation |
|--------------------|-------------|--------------------------------------------------------------------------------------------------|------------|
| `retryTimeout`      | [Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta) | Period to wait before each retry.                                                |            |
| `maxRetries`        | integer     | Maximum number of retries for the condition.                                                     |            |
| `type`              | string      | The name of the condition to wait for.                                                           |            |
| `message`           | string      | Optional message to match.                                                                      |            |
| `reason`            | string      | Optional reason to match.                                                                       |            |
| `transitionedAfter` | [Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta) | Minimum time since last transition to accept the condition.                      |            |
| `updatedAfter`      | [Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta) | Minimum time since last update to accept the condition.                          |            |

### `WaitStrategy`

**Used by:** [DestinationToWatch](#destinationtowatch)

| Field       | Type                                  | Description                                                | Validation |
|-------------|---------------------------------------|------------------------------------------------------------|------------|
| `time`      | [Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#duration-v1-meta) | Wait time between reconciliations.                         |            |
| `condition` | [WaitForCondition](#waitforcondition) | Condition that must be satisfied before continuing.         |            |

### `WebhookAuth`

WebhookAuth contains authentication methods for webhooks.

**Used by:** [WebhookConfig](#webhookconfig)

| Field         | Type                         | Description                                                    | Validation |
|---------------|------------------------------|----------------------------------------------------------------|------------|
| `basicAuth`   | [BasicAuth](#basicauth)      | Basic authentication credentials.                              |            |
| `bearerToken` | [BearerToken](#bearertoken)  | Kubernetes secret containing the bearer token.                 |            |

### `WebhookConfig`

WebhookConfig contains configuration for Webhook notifications.

**Used by:** [NotificationSource](#notificationsource)

| Field                    | Type                          | Description                                                                                           | Validation |
|--------------------------|-------------------------------|-------------------------------------------------------------------------------------------------------|------------|
| `path`                   | string                        | Endpoint path (default: `/webhook`). Always expects a POST request.                                   |            |
| `address`                | string                        | Address where the webhook is served. Defaults to `:8090`.                                             |            |
| `identifierPathOnPayload`| string                        | Key in the payload used to identify the secret. Defaults to `0.data.ObjectName` if not set.           |            |
| `webhookAuth`            | [WebhookAuth](#webhookauth)   | Authentication method for the webhook.                                                                |            |
| `retryPolicy`            | [RetryPolicy](#retrypolicy)   | Policy to retry failed messages. If not set, 4xx will be returned and no retry will be attempted.     |            |
