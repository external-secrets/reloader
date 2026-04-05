# Kubernetes ConfigMap

This guide explains how to set up Kubernetes ConfigMap events as a notification source for the Reloader component in your environment.

## Overview

The Reloader receives events from Kubernetes ConfigMap by using permissions from an authorized kubeconfig/service account to create a `watch` over ConfigMaps in the target cluster.

This works identically to the [Kubernetes Secret](kubernetes-secret.md) source, but watches ConfigMap resources instead.

## Prerequisites

* A valid service account / kubeconfig with `watch` permissions over ConfigMaps available in the cluster where `reloader` is installed.

## Step 1: Configure Reloader

Update your Reloader configuration to set up the Kubernetes ConfigMap listener.

### Configuration Spec

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-configmap-sample
spec:
  notificationSources:
    - type: KubernetesConfigMap
      kubernetesConfigMap:
        serverURL: https://kubernetes.default.svc
        # Auth configuration for the reloader
        # If targeting the same cluster, this block is optional
        auth:
          # b64 encoded CaBundle for the service.
          caBundle: Cg==
          # ref for a kubeconfig file in a form of a Kubernetes Secret
          kubeConfigRef:
            secretRef:
              name: reloader-kubeconfig
              key: kubeconfig
              namespace: reloader-system
          # ref for a token in a form of a Kubernetes Secret
          tokenRef:
            secretRef:
                name: reloader-token
                key: token
                namespace: reloader-system
          # ref for a Kubernetes Service Account
          serviceAccountRef:
            name: reloader
            namespace: reloader-system
        # Optional: narrow down which ConfigMaps to watch
        labelSelector:
          matchLabels:
            managed-by: reloader
  destinationsToWatch:
    - type: Deployment
      deployment:
        labelSelectors:
          matchLabels: {}
```

## Processing Events

Because reloader is only on a `watch` over ConfigMaps, it means that `get`, `list` and `watch` operations are not monitored.
