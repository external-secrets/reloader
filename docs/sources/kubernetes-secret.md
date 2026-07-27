# Kubernetes Secret

This guide explains how to set up Kubernetes Secret events as a notification source for the Reloader component in your environment.

## Overview

The Reloader receives events from Kubernetes Secret via using permissions an authorized kubeconfig/service account to create a `watch` over the Secrets in the target cluster.

## Prerequisites

* A valid service account / kubeconfig with `watch` permissions available in the cluster where `reloader` is installed.

## Step 1: Configure Reloader

Update your Reloader configuration to set up the Kubernetes Secret listener.

### Configuration Spec

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-kubernetes-sample
spec:
  notificationSources:
    - type: KubernetesSecret
      kubernetesSecret:
        serverURL: https://kubernetes.default.svc
        auth:
          caBundle: Cg==
          kubeConfigRef:
            secretRef:
              name: reloader-kubeconfig
              key: kubeconfig
              namespace: reloader-system
          tokenRef:
            secretRef:
                name: reloader-token
                key: token
                namespace: reloader-system
          serviceAccountRef:
            name: reloader
            namespace: reloader-system
  destinationsToWatch:
    - type: Deployment
      deployment:
        labelSelectors:
          matchLabels: {}
```

## Processing Events

Because reloader is only on a `watch` over Secrets, it means that `get` `list` and `watch` operations are not monitored.
