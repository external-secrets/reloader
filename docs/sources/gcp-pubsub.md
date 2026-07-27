# GCP PubSub

This guide explains how to set up GCP PubSub listener in order to allow Just In Time Rotation for GCP Secrets Manager.

## How it Works

`reloader` will listen for audit log events from a given GCP PubSub, and, based on pattern matching,
will trigger an `ExternalSecret` reconciliation if that object queries the GCP Secret Manager key that had its version updated.

## Setting Up GCP

In order to set up GCP, four steps are needed:

* Create a Topic and a Subscription on GCP Pub/Sub
* Create a Log Router to send information from Audit Logs to PubSub
* Create a Service Account & Permissions for `reloader`
* Install `reloader` in your cluster

!!! tip
    The following [terraform code](https://github.com/external-secrets-inc/public-examples/tree/main/pub-sub/terraform) contains a recipe for everything needed within GCP using service account keys. We recommend using it with a local `kind` cluster for a quick onboarding experience - use workload identity in production environments.

The only action remaining is to install `reloader` within your cluster.

??? example "Enable Service APIs"

    === "gcloud CLI"

        ```bash
        PROJECT_ID=your-project-id
        gcloud services enable pubsub.googleapis.com --project $PROJECT_ID
        gcloud services enable secretmanager.googleapis.com --project $PROJECT_ID
        gcloud services enable logging.googleapis.com --project $PROJECT_ID
        ```

    === "Terraform"

        ```hcl
        resource "google_project_service" "pubsub" {
          project = var.project_id
          service = "pubsub.googleapis.com"
        }

        resource "google_project_service" "secretmanager" {
          project = var.project_id
          service = "secretmanager.googleapis.com"
        }

        resource "google_project_service" "logging" {
          project = var.project_id
          service = "logging.googleapis.com"
        }
        ```

??? example "Create a Topic and a Subscription on GCP PubSub"

    === "gcloud CLI"

        ```bash
        PROJECT_ID=your-project-id
        TOPIC_ID=your-topic-id
        SUBSCRIPTION_ID=your-subscription-id
        gcloud pubsub topics create $TOPIC_ID --project $PROJECT_ID
        gcloud pubsub subscriptions create $SUBSCRIPTION_ID \
          --topic $TOPIC_ID \
          --project $PROJECT_ID
        ```

    === "Terraform"

        ```hcl
        variable "project_id" {
          type = string
        }

        variable "topic_name" {
          type    = string
          default = "your-topic-id"
        }

        variable "subscription_name" {
          type    = string
          default = "your-subscription-id"
        }

        provider "google" {
          project = var.project_id
          region  = "us-central1"
        }

        resource "google_pubsub_topic" "topic" {
          name = var.topic_name
        }

        resource "google_pubsub_subscription" "subscription" {
          name  = var.subscription_name
          topic = google_pubsub_topic.topic.name
        }
        ```

??? example "Create a Log Router for the PubSub Topic"

    === "gcloud CLI"

        ```bash
        PROJECT_ID=your-project-id
        TOPIC_ID=your-topic-id
        SUBSCRIPTION_ID=your-subscription-id
        SINK_NAME=secret-manager-addsecretversion-sink
        gcloud logging sinks create $SINK_NAME \
          pubsub.googleapis.com/projects/$PROJECT_ID/topics/$TOPIC_ID \
          --log-filter 'protoPayload.methodName=~"google.cloud.secretmanager.v1.SecretManagerService.AddSecretVersion"' \
          --project $PROJECT_ID
        SINK_SERVICE_ACCOUNT=$(gcloud logging sinks describe $SINK_NAME \
          --format 'value(writerIdentity)' \
          --project $PROJECT_ID)
        gcloud pubsub topics add-iam-policy-binding $TOPIC_ID \
          --member $SINK_SERVICE_ACCOUNT \
          --role roles/pubsub.publisher \
          --project $PROJECT_ID
        ```

    === "Terraform"

        ```hcl
        variable "sink_name" {
          type    = string
          default = "secret-manager-addsecretversion-sink"
        }

        resource "google_logging_project_sink" "log_sink" {
          name        = var.sink_name
          destination = "pubsub.googleapis.com/projects/${var.project_id}/topics/${google_pubsub_topic.topic.name}"
          filter      = <<EOF
        protoPayload.methodName=~"google.cloud.secretmanager.v1.SecretManagerService.AddSecretVersion"
        EOF
        }

        resource "google_pubsub_topic_iam_member" "sink_publisher" {
          topic  = google_pubsub_topic.topic.name
          role   = "roles/pubsub.publisher"
          member = google_logging_project_sink.log_sink.writer_identity
        }
        ```

??? example "Create Service Account & Permissions (Service Account Keys)"

    === "gcloud CLI"

        ```bash
        SA_NAME=reloader-sa
        PROJECT_ID=your-project-id
        gcloud iam service-accounts create $SA_NAME \
          --display-name "Service Account for Reloader" \
          --project $PROJECT_ID
        gcloud projects add-iam-policy-binding $PROJECT_ID \
          --member "serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
          --role roles/pubsub.subscriber
        gcloud iam service-accounts keys create key.json \
          --iam-account "$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"
        echo "now, make sure you install this key in the cluster so that reloader can use it"
        ```

    === "Terraform"

        ```hcl
        variable "service_account_name" {
          type    = string
          default = "reloader-sa"
        }

        resource "google_service_account" "reloader_sa" {
          account_id   = var.service_account_name
          display_name = "Service Account for Reloader"
        }

        resource "google_project_iam_member" "pubsub_subscriber_sa_key" {
          project = var.project_id
          role    = "roles/pubsub.subscriber"
          member  = "serviceAccount:${google_service_account.reloader_sa.email}"
        }

        resource "google_project_iam_member" "secret_accessor_sa_key" {
          project = var.project_id
          role    = "roles/secretmanager.secretAccessor"
          member  = "serviceAccount:${google_service_account.reloader_sa.email}"
        }

        resource "google_service_account_key" "sa_key" {
          service_account_id = google_service_account.reloader_sa.name
          public_key_type    = "TYPE_X509_PEM_FILE"
          private_key_type   = "TYPE_GOOGLE_CREDENTIALS_FILE"
        }
        ```

??? example "Create Service Account & Permissions (Workload Identity)"

    === "gcloud CLI"

        ```bash
        SA_NAME=reloader-sa
        PROJECT_ID=your-project-id
        NAMESPACE=service-account-namespace
        KSA_NAME=service-account-name
        gcloud iam service-accounts create $SA_NAME \
          --display-name "Service Account for Reloader" \
          --project $PROJECT_ID
        gcloud projects add-iam-policy-binding $PROJECT_ID \
          --member "serviceAccount:$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
          --role roles/pubsub.subscriber
        gcloud iam service-accounts add-iam-policy-binding $SA_NAME@$PROJECT_ID.iam.gserviceaccount.com \
          --member "serviceAccount:$PROJECT_ID.svc.id.goog[$NAMESPACE/$KSA_NAME]" \
          --role roles/iam.workloadIdentityUser
        kubectl create serviceaccount $KSA_NAME --namespace $NAMESPACE
        kubectl annotate serviceaccount $KSA_NAME \
          --namespace $NAMESPACE \
          iam.gke.io/gcp-service-account=$SA_NAME@$PROJECT_ID.iam.gserviceaccount.com
        ```

    === "Terraform"

        ```hcl
        variable "kubernetes_service_account_name" {
          type    = string
          default = "reloader-ksa"
        }

        variable "namespace" {
          type    = string
          default = "your-namespace"
        }

        variable "cluster_name" {
          type = string
        }

        variable "cluster_location" {
          type = string
        }

        provider "kubernetes" {
          host  = google_container_cluster.gke_cluster.endpoint
          token = data.google_client_config.default.access_token
          cluster_ca_certificate = base64decode(
            google_container_cluster.gke_cluster.master_auth[0].cluster_ca_certificate,
          )
        }

        data "google_client_config" "default" {}

        resource "kubernetes_service_account" "ksa" {
          metadata {
            name      = var.kubernetes_service_account_name
            namespace = var.namespace
            annotations = {
              "iam.gke.io/gcp-service-account" = google_service_account.reloader_sa.email
            }
          }
        }

        resource "google_service_account" "reloader_sa" {
          account_id   = var.service_account_name
          display_name = "Service Account for Reloader"
        }

        resource "google_service_account_iam_member" "workload_identity_binding" {
          service_account_id = google_service_account.reloader_sa.name
          role               = "roles/iam.workloadIdentityUser"
          member             = "serviceAccount:${var.project_id}.svc.id.goog[${var.namespace}/${var.kubernetes_service_account_name}]"
        }

        resource "google_project_iam_member" "pubsub_subscriber_wi" {
          project = var.project_id
          role    = "roles/pubsub.subscriber"
          member  = "serviceAccount:${google_service_account.reloader_sa.email}"
        }

        resource "google_project_iam_member" "secret_accessor_wi" {
          project = var.project_id
          role    = "roles/secretmanager.secretAccessor"
          member  = "serviceAccount:${google_service_account.reloader_sa.email}"
        }
        ```

After all these steps - we are ready to install & use `reloader`.

### Example Configuration

!!! tip
    Before applying this manifest, be sure to have [installed reloader](../quickstart.md) first.

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: gcp-sample
spec:
  notificationSources:
    - type: GooglePubSub
      googlePubSub:
        subscriptionID: value-from-subscription-id
        projectID: value-from-project-id
        auth:
        # If using service account keys
          secretRef:
            secretAccessKeySecretRef:
              name: secret-name
              namespace: secret-namespace
              key: creds_json
        # If using Workload Identity
          workloadIdentity:
            clusterName: name-of-your-cluster-if-on-different-project
            clusterLocation: us-east1
            clusterProjectID: cluster-project-if-different
            serviceAccountRef:
              name: service-account-name
              namespace: service-account-namespace
  destinationsToWatch:
    - type: ExternalSecret
      externalSecret:
        labelSelectors:
          matchLabels:
            app: my-app
```

After that, every new `AddSecretVersion` message will automatically update your `Kubernetes Secret` values!
