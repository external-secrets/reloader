# Quickstart

Reloader is a tool that allows to trigger kubernetes manifests updates based on events from different sources:

* [GCP Pubsub](sources/gcp-pubsub.md)
* [AWS SQS](sources/aws-sqs.md)
* [Azure EventGrid](sources/azure-eventgrid.md)
* [Hashicorp Vault audit Logs](sources/vault.md)
* [Generic Webhook](sources/webhook.md)
* [Kubernetes Secret](sources/kubernetes-secret.md)

With it, it is possible to trigger manifest changes to multiple destinations:

* [ExternalSecret](https://external-secrets.io/latest/introduction/getting-started/)
* [Deployments](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)

And many more to come!

## Installing Reloader

### Install a manifest bundle

Install the manifest in your desired cluster with `kubectl apply`:

```bash
VERSION=<reloader_version>
curl -L https://github.com/external-secrets/reloader/releases/download/$VERSION/bundle.yaml | kubectl apply -f -
```

## Configure Reloading Deployments on Secret Changes

```yaml
cat << EOF | kubectl apply -f -
apiVersion: reloader.external-secrets.io/v1alpha1
## Config is a Cluster Scoped resource for reloader configuration
kind: Config
metadata:
  name: reloader-sample
  labels:
    app.kubernetes.io/name: reloader
spec:
  notificationSources:
    - type: KubernetesSecret
      kubernetesSecret:
        ## Watch secrets internal to the cluster
        serverURL: https://kubernetes.default.svc
  destinationsToWatch:
    - type: Deployment
      deployment:
        labelSelectors:
          matchLabels: {}
EOF
```

## Testing it out

Let's first create two deployments and a Secret:

```yaml
cat << EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: one
  name: one
spec:
  replicas: 1
  selector:
    matchLabels:
      app: one
  template:
    metadata:
      labels:
        app: one
    spec:
      containers:
      - command:
        - sh
        - -c
        - sleep 3600
        env:
        - name: TEST
          valueFrom:
            secretKeyRef:
              key: token
              name: test
        image: ubuntu
        imagePullPolicy: Always
        name: ubuntu
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: two
  name: two
spec:
  replicas: 1
  selector:
    matchLabels:
      app: two
  template:
    metadata:
      labels:
        app: two
    spec:
      containers:
      - command:
        - sh
        - -c
        - sleep 3600
        env:
        - name: TEST
          valueFrom:
            secretKeyRef:
              key: token
              name: test
        image: ubuntu
        imagePullPolicy: Always
        name: ubuntu
---
apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  token: dGhpcy1pcy1hLXRva2Vu # this-is-a-token
EOF
```

Now, let's rotate the secret value:

```bash
kubectl patch secret test -p '{"data":{"token":"bmV3LXRva2VuLXZhbHVl"}}'
```

Now watch deployments get restarted in sequence and enjoy!!

## Other notes

### Install with Helm Chart

!!! note
    The helm chart below is an example for the community to use as a baseline.
    It isn't part of our release and should not be considered ready for production use.

In reloader repositories, a sample helm chart is contained to help you install it with different tooling.
In order to use it, you can simply do:

```bash
git clone https://github.com/external-secrets/reloader
helm install reloader -n reloader --create-namespace ./examples/helm-chart/reloader
```

## Next Steps

* Choose a notification source that will trigger secret rotations for you
* Configure the notification source and get your secrets rotating
* Configure the destination you want to use
* Make your rotation event driven!

## Support

For any bugs or feature requests, you can go to [GitHub](https://github.com/external-secrets/reloader/issues).

If you need support for your specific use case, contact us via [Slack](https://kubernetes.slack.com/archives/C017BF84G2Y).
