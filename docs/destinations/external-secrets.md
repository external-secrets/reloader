# ExternalSecrets

## Overview

An ExternalSecret destination consists of a Kubernetes ExternalSecret Custom Resource that will be triggered to roll out when an event arrives.

In order to use this destination, [external-secrets operator](https://github.com/external-secrets/external-secrets) must be installed in the cluster.

This can be used with the `Secret` Notification source to automatically trigger a ExternalSecrets reconciliation when a secret changes.

## Configuration

ExternalSecret configuration is straightforward: all you need to do is add a `externalSecret` as destination to Watch in your configuration manifest:

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
    - type: ExternalSecret
      externalSecret:
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
