# PushSecrets

## Overview

A PushSecret destination consists of a Kubernetes PushSecret Custom Resource that will be triggered to reconcile when an event arrives.

In order to use this destination, [external-secrets operator](https://github.com/external-secrets/external-secrets) must be installed in the cluster.

This can be used with any notification source to automatically trigger a PushSecret reconciliation when a secret changes.

- Default **UpdateStrategy** is annotations patch to trigger PushSecret reconcile.
- Default **MatchStrategy** matches secret keys using:
    - `spec.selector.secret.name`
    - `spec.data[*].match.remoteRef.remoteKey`

## Configuration

PushSecret configuration is straightforward: all you need to do is add a `pushSecret` as a destination to Watch in your configuration manifest:

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
    - type: PushSecret
      pushSecret:
        # you may set one or more of the below:
        # namespace selectors restrict PushSecret destinations to be on a given namespace set.
        namespaceSelectors:
          matchLabels:
            specific-namespaces-auto-rollout: allowed
        # label selectors target specific PushSecrets within restricted namespaces.
        labelSelectors:
          matchLabels:
            specific-pushsecret-labels-match: "true"
        # Filters down to specific PushSecret names
        names:
        - optional-name-matching-for-each-pushsecret
```
