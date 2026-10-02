# ExternalDNS - Yandex Cloud DNS Webhook

This is an [ExternalDNS provider](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/webhook-provider.md) for [Yandex Cloud DNS](https://cloud.yandex.com/en/services/dns).
This projects externalizes the provider for Yandex Cloud DNS and offers a way forward for bugfixes.

## Installation

This webhook provider is run easiest as sidecar within the `external-dns` pod. This can be achieved using the official
`external-dns` Helm chart and [its support for the `webhook` provider type]([https://kubernetes-sigs.github.io/external-dns/latest/charts/external-dns/#providers]).

Setting the `provider.name` to `webhook` allows configuration of the
`external-dns-yandex-webhook` via a few additional values:

```yaml
provider:
  name: webhook
  webhook:
    image:
      repository: ghcr.io/ismailbaskin/external-dns-yandex-webhook
      tag: 1.0.0
    args:
      - --folder-id=YOUR_FOLDER_ID
      - --auth-key-file=/etc/kubernetes/key.json
    extraVolumeMounts:
      - name: yandexconfig
        mountPath: /etc/kubernetes/
    resources: {}
    securityContext:
      runAsUser: 1000
```

The referenced `extraVolumeMount` points to a `Secret` containing the service account key file for Yandex Cloud authentication.

## Command Line Arguments

| Argument | Description | Default |
| --- | --- | --- |
| `--folder-id` | Yandex Cloud folder ID where your DNS zones are located. Required. | None |
| `--auth-key-file` | Path to the Yandex Cloud service account key file. Required unless workload identity is used. | None |
| `--use-workload-identity` | Use workload identity for authentication. Cannot be used with `--auth-key-file`. | `false` |
| `--network-interface` | Network interface on which the webhook server listens. | `0.0.0.0` |
| `--webhook-port` | Port on which the webhook server listens. | `8888` |
| `--health-port` | Port on which the health check server listens. | `8080` |

Provide either `--auth-key-file` or `--use-workload-identity`.

## Authentication

Create a service account in Yandex Cloud with the necessary permissions for DNS management

### Service account key

For authentication, with a service account key file:

1. Create a service account key using the Yandex Cloud CLI:

```shell
# Install Yandex Cloud CLI if you haven't already
# https://cloud.yandex.com/en/docs/cli/quickstart

# Create the IAM key JSON file
yc iam key create iamkey \
  --service-account-id=<your service account ID> \
  --format=json \
  --output=key.json
```

2. Add this file to your Kubernetes Secret

Create a Secret with the service account key file:

```shell
kubectl create secret generic yandexconfig --namespace external-dns --from-file=key.json
```

and then add it as an extraVolume to within the `values.yaml` of external-dns:

```yaml
extraVolumes:
  - name: yandexconfig
    secret:
      secretName: yandexconfig
```

### Workload identity

For authentication, with workload identity

1. Add yandex iam workload identity oidc federation for k8s cluster

```
ISSUER_URL="https://storage.yandexcloud.net/mk8s-oidc/v1/clusters/<cluster-id>"
JWKS_URL="${ISSUER_URL}/jwks.json"

yc iam workload-identity oidc federation create \
  --name my-k8s-federation \
  --issuer "${ISSUER_URL}" \
  --audiences "${ISSUER_URL}" \
  --jwks-url "${JWKS_URL}"
```

2. create a federated credential for Yandex IAM service account with appropriate dns permissions

```
FEDERATION_ID=$(
  yc iam workload-identity oidc federation get my-k8s-federation \
    --format json |
  jq -r '.id'
)
yc iam workload-identity federated-credential create \
  --service-account-id <YC_SERVICE_ACCOUNT_ID> \
  --federation-id "${FEDERATION_ID}" \
  --external-subject-id \
    "system:serviceaccount:<K8S_NAMESPACE>:<K8S_SERVICE_ACCOUNT>"
```

3. Add annotation to k8s service account that is used by external dns pod

```
  annotations:
    yandex.cloud/federated-yc-service-account-id: <YC_SERVICE_ACCOUNT_ID>
```
