# Argo Rollouts install

```sh
kubectl create namespace argo-rollouts || true
kubectl apply -n argo-rollouts \
  -f https://github.com/argoproj/argo-rollouts/releases/download/v1.7.2/install.yaml
kubectl -n argo-rollouts wait --for=condition=Available deploy --all --timeout=180s
```

CLI plugin (推奨):
```sh
curl -LO https://github.com/argoproj/argo-rollouts/releases/download/v1.7.2/kubectl-argo-rollouts-darwin-arm64
chmod +x kubectl-argo-rollouts-darwin-arm64
sudo mv kubectl-argo-rollouts-darwin-arm64 /usr/local/bin/kubectl-argo-rollouts
```
