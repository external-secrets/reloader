# AWS SQS

This guide explains how to set up the AWS Simple Queue Service (SQS) notification source for the Reloader component in your environment. Using AWS SQS allows you to trigger secret rotation events based on messages sent to your SQS queue.

## Overview

The Reloader listens to AWS SQS messages to determine when to trigger secret rotations. The overall process is as follows:

1. **Secrets Stored in AWS Secrets Manager**: Your secrets are stored and managed in AWS Secrets Manager.
2. **EventBridge Configuration**: AWS EventBridge is configured to publish events related to secret changes to an SQS queue.
3. **Reloader Listener**: The Reloader listens to the SQS queue for events and annotates the corresponding Kubernetes ExternalSecrets to trigger reconciliation and synchronization of the secret values.

This guide will walk you through configuring AWS EventBridge and SQS to work with the Reloader.

## Prerequisites

- An AWS account with permissions to manage Secrets Manager, SQS, and EventBridge.
- Kubernetes cluster with Reloader installed.
- AWS IAM roles and permissions set up for authentication (either via credentials or IRSA).
- If you're targeting an `ExternalSecret` destination, [external-secrets](https://github.com/external-secrets/external-secrets) operator needs to be installed.

!!! tip
    The following [terraform code](https://github.com/external-secrets-inc/public-examples) contains a recipe for everything needed within AWS using service account keys. If you plan to use static authentication credentials (access key ID and secret access key), we recommend using a local `kind` cluster for a quick onboarding experience. If you intend to use service accounts with IRSA enabled, you'll need an EKS cluster with an associated OIDC provider.

## Step 1: Configure AWS SQS Queue

Create an AWS SQS queue that will receive events from AWS Secrets Manager via EventBridge.

### Create an SQS Queue

1. Sign in to the AWS Management Console and navigate to **SQS**.
2. Click on **Create queue**.
3. Choose the queue type (**Standard** or **FIFO**) based on your requirements.
4. Configure the queue settings as needed.
5. Note down the **Queue URL**; you will need it later.

## Step 2: Configure AWS EventBridge Rule

Set up an EventBridge rule to forward events from AWS Secrets Manager to your SQS queue.

### Create an EventBridge Rule

1. Navigate to **EventBridge** in the AWS Management Console.
2. Click on **Rules** in the sidebar, then **Create rule**.
3. Enter a **Name** and **Description** for your rule.
4. Under **Define pattern**, select **Event pattern**.
5. Choose **AWS events or EventBridge partner events**.
6. In **Event source**, select **AWS services**.
7. In **AWS service**, select **Secrets Manager**.
8. In **Event type**, select **AWS API Call via CloudTrail**.

### Specify Event Pattern

Under **Event pattern**, specify the events you want to capture.

```json
{
  "source": ["aws.secretsmanager"],
  "detail-type": ["AWS API Call via CloudTrail"],
  "detail": {
    "eventSource": ["secretsmanager.amazonaws.com"],
    "eventName": ["PutSecretValue", "UpdateSecret", "RotationSucceeded"]
  }
}
```

### Configure Target

1. Under **Select targets**, choose **SQS queue**.
2. Select the SQS queue you created earlier.
3. Configure any additional settings as needed.
4. Click **Create** to save the rule.

## Step 3: Configure Reloader

Update your Reloader configuration to connect to the SQS queue.

### Authentication

The Reloader needs to authenticate with AWS to access the SQS queue. You can use either AWS credentials stored in Kubernetes Secrets or a Kubernetes ServiceAccount configured with IRSA.

### Using AWS Credentials from a Kubernetes Secret

- **`authMethod`**: `credentials`
- **`secretRef`**: References to Kubernetes Secrets containing AWS credentials:
    - **`accessKeyIDSecretRef`**: Reference to the AWS Access Key ID.
    - **`secretAccessKeySecretRef`**: Reference to the AWS Secret Access Key.
    - **`sessionTokenSecretRef`**: *(Optional)* Reference to the AWS session token for temporary credentials.

#### Example

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-aws-sqs
spec:
  notificationSources:
    - type: AwsSqs
      awsSqs:
        queueURL: https://sqs.us-east-1.amazonaws.com/123456789012/my-queue
        region: us-east-1
  auth:
    authMethod: credentials
    secretRef:
      accessKeyIDSecretRef:
        name: aws-credentials
        key: accessKey
      secretAccessKeySecretRef:
        name: aws-credentials
        key: secretKey
      sessionTokenSecretRef:
        name: aws-session
        key: token
  destinationsToWatch:
    - type: ExternalSecret
      externalSecret:
        labelSelectors:
          matchLabels:
            app: my-app
```

### Using a Kubernetes ServiceAccount

If the listener is configured with the `serviceAccount` auth method, then IAM Roles for Service Accounts (IRSA) needs to be set up. IRSA allows you to associate an AWS IAM role with a Kubernetes ServiceAccount, enabling the pod to access AWS resources securely using OpenID Connect (OIDC).

#### Brief Overview of IRSA and OIDC

- **IRSA**: Integrates Kubernetes ServiceAccounts with AWS IAM roles. This allows pods running in Kubernetes to assume IAM roles and access AWS resources without managing AWS credentials manually.
- **OIDC**: An identity layer on top of the OAuth 2.0 protocol, used by IRSA to authenticate Kubernetes ServiceAccounts with AWS IAM.

By configuring IRSA, each listener can work with a separate Kubernetes ServiceAccount mapped to different IAM roles. This provides fine-grained access control and follows the principle of least privilege.

#### Example Configuration

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-aws-irsa
spec:
  notificationSources:
    - type: AwsSqs
      awsSqs:
        queueURL: https://sqs.us-east-1.amazonaws.com/123456789012/my-queue
        region: us-east-1
        auth:
          authMethod: serviceAccount
          serviceAccountRef:
            name: my-service-account
        maxNumberOfMessages: 10
        waitTimeSeconds: 20
        visibilityTimeout: 30
  destinationsToWatch:
    - type: ExternalSecret
      externalSecret:
        labelSelectors:
          matchLabels:
            app: my-app
```

## Step 4: Update IAM Permissions

Ensure that the IAM role associated with your authentication method has the necessary permissions to:

- Receive messages from the SQS queue.

## Step 5: Deploy the Reloader

Apply the Reloader configuration to your Kubernetes cluster:

```bash
kubectl apply -f reloader-sample.yaml
```

## Processing SQS Messages

When a secret is updated in AWS Secrets Manager, an event is sent to the SQS queue via EventBridge. The Reloader listens to the queue, processes the messages, and annotates the corresponding Kubernetes ExternalSecrets to trigger a reconciliation.

### Default AWS SQS Message Schema

```json
{
  "version": "0",
  "id": "79f04f06-924c-7a00-b3f8-e532167d23e0",
  "detail-type": "AWS API Call via CloudTrail",
  "source": "aws.secretsmanager",
  "account": "123456789012",
  "time": "2024-10-23T12:26:45Z",
  "region": "eu-west-1",
  "resources": [],
  "detail": {
    "eventVersion": "1.09",
    "eventTime": "2024-10-23T12:26:45Z",
    "eventName": "PutSecretValue",
    "awsRegion": "eu-west-1",
    "userAgent": "Mozilla/5.0",
    "requestParameters": {
      "secretId": "arn:aws:secretsmanager:eu-west-1:123456789012:secret:prod/test-secret-123456",
      "clientRequestToken": "4d383ea9-084a-49c9-8722-cbfb0f25c456"
    },
    "responseElements": {
      "arn": "arn:aws:secretsmanager:eu-west-1:123456789012:secret:prod/test-secret-123456"
    },
    "eventID": "12345678-1234-1234-1234-123456789012",
    "readOnly": false,
    "eventType": "AwsApiCall",
    "managementEvent": true,
    "recipientAccountId": "123456789012",
    "eventCategory": "Management",
    "tlsDetails": {
      "tlsVersion": "TLSv1.3",
      "cipherSuite": "TLS_AES_128_GCM_SHA256",
      "clientProvidedHostHeader": "secretsmanager.eu-west-1.amazonaws.com"
    }
  }
}
```

The Reloader looks for the `eventName` and `requestParameters.secretId` fields in the message payload to identify which secret has changed.

## Additional Configuration Options

- **`maxNumberOfMessages`**: *(Optional)* The maximum number of messages to retrieve from the SQS queue in a single request. Adjust based on your application's throughput requirements.
- **`waitTimeSeconds`**: *(Optional)* The duration (in seconds) to wait for messages if the queue is empty. Increasing this value can reduce the number of empty responses when polling.
- **`visibilityTimeout`**: *(Optional)* The duration (in seconds) that a received message is hidden from other consumers. Set this value based on how long it takes to process a message.

## Summary

By configuring AWS EventBridge to send events to an SQS queue and setting up the Reloader to listen to that queue, you enable automatic synchronization of secret updates from AWS Secrets Manager to your Kubernetes cluster.

## Additional Resources

- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [AWS EventBridge Documentation](https://docs.aws.amazon.com/eventbridge/)
- [AWS SQS Documentation](https://docs.aws.amazon.com/sqs/)
- [AWS IAM Roles for Service Accounts (IRSA)](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)
- [External Secrets Operator Documentation](https://external-secrets.io/)
