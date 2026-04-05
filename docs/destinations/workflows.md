# WorkflowRunTemplate

## Overview

A WorkflowRunTemplate destination consists of a Kubernetes WorkflowRunTemplate Custom Resource that will be triggered to roll out when an event arrives.

This can be used with any notification source to automatically trigger a WorkflowRunTemplate reconciliation when a relevant event is received.

## Configuration

WorkflowRunTemplate configuration is straightforward: all you need to do is add a `workflowRunTemplate` as a destination to Watch in your configuration manifest:

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
    - type: WorkflowRunTemplate
      workflowRunTemplate:
        # you may set one or more of the above:
        # namespace selectors restrict WorkflowRunTemplate destinations to be on a given namespace set.
        namespaceSelectors:
          matchLabels:
            specific-namespaces-auto-rollout: allowed
        # label selectors target specific WorkflowRunTemplate within restricted namespaces.
        labelSelectors:
          matchLabels:
            specific-workflow-labels-match: "true"
        # Filters down to specific WorkflowRunTemplate names
        names:
        - optional-name-matching-for-each-workflow
```
