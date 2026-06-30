# vcluster overlay

Helm でデプロイ:
```sh
helm upgrade --install vc-a vcluster -n vc-a --create-namespace --repo https://charts.loft.sh -f values-vc-a.yaml
helm upgrade --install vc-b vcluster -n vc-b --create-namespace --repo https://charts.loft.sh -f values-vc-b.yaml
vcluster connect vc-a -n vc-a -- kubectl get ns
```

vc-a と vc-b は host cluster から見ると独立した namespace、内側は別 cluster。
