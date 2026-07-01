# Flux install

## CLI 導入

```sh
brew install fluxcd/tap/flux    # macOS
# OR
curl -s https://fluxcd.io/install.sh | bash
```

## Cluster にインストール

```sh
kubectl config use-context kind-learning-base
flux install
```

`flux-system` namespace に 4 controller (source, kustomize, helm, notification) が立つ。
