# Argo Events install

```sh
kubectl create namespace argo-events || true
kubectl apply -n argo-events \
  -f https://raw.githubusercontent.com/argoproj/argo-events/v1.9.2/manifests/install.yaml
kubectl apply -n argo-events \
  -f https://raw.githubusercontent.com/argoproj/argo-events/v1.9.2/manifests/install-validating-webhook.yaml
kubectl -n argo-events wait --for=condition=Available deploy --all --timeout=300s
```
