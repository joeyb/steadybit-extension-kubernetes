<img src="./logo.png" height="130" align="right" alt="Kubernetes logo depicting a helm next to text 'Kubernetes'">

# Steadybit extension-kubernetes

A [Steadybit](https://www.steadybit.com/) extension implementation for Kubernetes.

Learn about the capabilities of this extension in our [Reliability Hub](https://hub.steadybit.com/extension/com.steadybit.extension_kubernetes).

## Configuration

| Environment Variable                             | Helm value                  | Meaning                                                                   | required | default |
|--------------------------------------------------|-----------------------------|---------------------------------------------------------------------------|----------|---------|
| `STEADYBIT_EXTENSION_KUBERNETES_CLUSTER_NAME`    | `kubernetes.clusterName`    | The name of the kubernetes cluster                                        | yes      |         |
| `STEADYBIT_EXTENSION_DISABLE_DISCOVERY_EXCLUDES` | `discovery.disableExcludes` | Ignore discovery excludes specified by `steadybit.com/discovery-disabled` | false    | `false` |
| `STEADYBIT_EXTENSION_LABEL_FILTER`               |                             | These labels will be ignored and not added to the discovered targets      | false    | `false` |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

The process requires access rights to interact with the Kubernetes API.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: extension-kubernetes
rules:
  - apiGroups:
      - apps
    resources:
      - deployments
      - replicasets
      - daemonsets
      - statefulsets
    verbs:
      - get
      - list
      - watch
      - patch
  - apiGroups: [""]
    resources:
      - services
      - pods
      - nodes
      - events
    verbs:
      - get
      - list
      - watch
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: extension-kubernetes
  namespace: steadybit-extension
automountServiceAccountToken: true
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: extension-kubernetes
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: extension-kubernetes
subjects:
  - kind: ServiceAccount
    name: extension-kubernetes
    namespace: steadybit-extension
```

## Installation

We recommend that you deploy the extension with
our [official Helm chart](https://github.com/steadybit/extension-kubernetes/tree/main/charts/steadybit-extension-kubernetes).

### Helm

```sh
helm repo add steadybit-extension-kubernetes https://steadybit.github.io/extension-kubernetes
helm repo update

helm upgrade steadybit-extension-kubernetes \
  --install \
  --wait \
  --timeout 5m0s \
  --create-namespace \
  --namespace steadybit-extension \
  --set kubernetes.clusterName=<NAME_OF_YOUR_CLUSTER> \
  steadybit-extension-kubernetes/steadybit-extension-kubernetes
```

### Docker

You may alternatively start the Docker container manually.

```sh
docker run \
  --env STEADYBIT_LOG_LEVEL=info \
  --expose 8088 \
  --env STEADYBIT_EXTENSION_KUBERNETES_CLUSTER_NAME=<NAME_OF_YOUR_CLUSTER> \
  ghcr.io/steadybit/extension-kubernetes:latest
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.

## mark resources as "do not discover"

to exclude a deployment / namespace / pod from discovery you can add the label `"steadybit.com/discovery-disabled": "true"` to the resource labels
