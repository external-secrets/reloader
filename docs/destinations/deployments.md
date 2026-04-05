# Deployments

## Overview

A Deployment destination consists of a Kubernetes Deployment resource that will be triggered to roll out when an event arrives.

This can be used with the `Secret` Notification source to automatically trigger a deployment rollout when a secret changes.

## Configuration

Deployment configuration is straightforward: all you need to do is add a `deployment` as destination to Watch in your configuration manifest:

```yaml
apiVersion: reloader.external-secrets.io/v1alpha1
kind: Config
metadata:
  name: reloader-sample
  labels:
    app.kubernetes.io/name: reloader
spec:
  notificationSources:
  - #... any notification sources you want
  destinationsToWatch:
    - type: Deployment
      deployment:
        # you may set one or more of the above:
        # namespace selectors restrict deployment destinations to be on a given namespace set.
        namespaceSelectors:
          matchLabels:
            specific-namespaces-auto-rollout: allowed
        # label selectors target specific deployment within restricted namespaces.
        labelSelectors:
          matchLabels:
            specific-deployment-labels-match: "true"
        # Filters down to specific deployment names
        names:
        - optional-name-matching-for-each-deployment
```
