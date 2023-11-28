# ExternalDNS - Bizfly Cloud Webhook

**⚠️ NOTE**: his Webhook was forked and modified from the IONOS Webhook to work with Bizfly Cloud:
https://github.com/ionos-cloud/external-dns-ionos-webhook

ExternalDNS is a Kubernetes add-on for automatically managing
Domain Name System (DNS) records for Kubernetes services by using different DNS providers.
By default, Kubernetes manages DNS records internally,
but ExternalDNS takes this functionality a step further by delegating the management of DNS records to an external DNS
provider such as Bizfly Cloud.  Therefore, the Bizfly Cloud webhook allows to manage your
Bizfly Cloud domains inside your kubernetes cluster with [ExternalDNS](https://github.com/kubernetes-sigs/external-dns).

To use ExternalDNS with Bizfly Cloud, you need a Bizfly Cloud API key with permissions to create and modify DNS records.

### Deployment in Kubernetes
secret.yml
```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: external-dns-bizflycloud
stringData:
  credential_id: "xxx"
  credential_secret: "xxx"
```

The Bizfly Cloud API is a RESTful API based on HTTPS requests and JSON responses. If you are registered with Bizfly Cloud, you can create your credential_id, credential_secret from [here](https://manage.bizflycloud.vn/account/configuration/credential).

`$ kubectl apply -f secret.yaml`

external-dns-bizflycloud.yaml

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: external-dns
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: external-dns
rules:
  - apiGroups: [""]
    resources: ["services","endpoints","pods"]
    verbs: ["get","watch","list"]
  - apiGroups: ["extensions","networking.k8s.io"]
    resources: ["ingresses"]
    verbs: ["get","watch","list"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: external-dns-viewer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: external-dns
subjects:
  - kind: ServiceAccount
    name: external-dns
    namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: external-dns
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: external-dns
  template:
    metadata:
      labels:
        app: external-dns
    spec:
      serviceAccountName: external-dns
      containers:
        - name: external-dns
          image: registry.k8s.io/external-dns/external-dns:v0.14.0
          args:
            - --source=service
            - --source=ingress
            - --provider=webhook

        - image: ghcr.io/bizflycloud/external-dns-bizflycloud-webhook:v0.1.0
          name: bizflycloud-webhook
          ports:
            - containerPort: 8888
          env:
            - name: BFC_APP_CREDENTIAL_ID
              valueFrom:
                secretKeyRef:
                  name: external-dns-bizflycloud
                  key: credential_id
            - name: BFC_APP_CREDENTIAL_SECRET
              valueFrom:
                secretKeyRef:
                  name: external-dns-bizflycloud
                  key: credential_secret
```
`$ kubectl apply -f external-dns-bizflycloud.yaml`


Example nginx deployment using the external DNS:

nginx.yaml

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - image: nginx
          name: nginx
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    external-dns.alpha.kubernetes.io/hostname: bfcexample.com.
    external-dns.alpha.kubernetes.io/ttl: "120" #optional

spec:
  selector:
    app: nginx
  type: LoadBalancer
  ports:
    - protocol: TCP
      port: 80
      targetPort: 80
```

`$ kubectl apply -f nginx.yaml`

### Local deployment
```bash
export BFC_APP_CREDENTIAL_ID=xxx
export BFC_APP_CREDENTIAL_SECRET=xxx
```

```bash

make run

# In another terminal
curl http://localhost:8888/records -H 'Accept: application/external.dns.webhook+json;version=1'
# Example response:
[
  {
    "dnsName": "nginx.internal",
    "targets": [
      "10.99.237.62"
    ],
    "recordType": "A",
    "recordTTL": 3600
  }
]
```

## How To Contribute

Development happens at GitHub; any typical workflow using Pull Requests are welcome. In the same spirit, we use the GitHub issue tracker for all reports (regardless of the nature of the report, feature request, bugs, etc.).
