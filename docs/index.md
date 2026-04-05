# External Secrets Reloader

External Secrets Reloader allows you to trigger updates on destinations based on events from different sources.

- [Source Code](https://github.com/external-secrets/reloader)
- [Slack Channel](https://kubernetes.slack.com/archives/C017BF84G2Y)

## Design Overview

![Design Overview](diagram.excalidraw.png)

## Why use Reloader?

You should use reloader if you want a simple way to:

* Trigger a deployment Rollout on Secret Changes
* Trigger external-secrets reconcile on a Cloud Event
* Trigger external-secrets reconcile on a generic webhook
* Trigger external-secrets reconcile on a Kubernetes Secret change (for kubernetes SecretStores)

## Next Steps

[Get Started with a hands-on example](quickstart.md)
