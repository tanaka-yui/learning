# vCluster — 仮想クラスタ・syncer・テナント隔離

## vCluster とは

**vCluster** はホスト Kubernetes クラスタの上に **仮想 Kubernetes クラスタ** を作るツールである。
vCluster ごとに独立した API サーバ・etcd・controller manager が起動し、テナントには完全な Kubernetes クラスタに見える。

```
ホストクラスタ (kind-learning-base)
  └── Namespace: vc-a
        └── vCluster Pod (API server + syncer)
              └── 仮想クラスタ A（テナント A が操作）
  └── Namespace: vc-b
        └── vCluster Pod (API server + syncer)
              └── 仮想クラスタ B（テナント B が操作）
```

## syncer の仕組み

`syncer` は vCluster の核となるコンポーネントで、仮想クラスタのリソースをホストクラスタに同期する。

```
仮想クラスタ (API server)
  │
  │  Pod / Service / ConfigMap / PVC ...
  ▼
syncer
  │  translate (namespace prefix を付与)
  ▼
ホストクラスタ (API server)
  └── Namespace: vc-a
        └── Pod: my-pod → vc-a-my-pod-xxxxx
```

テナントが `kubectl apply` した Pod はホストの Namespace `vc-a` 内の Pod として実際に起動する。テナントからはホストの他 Namespace が見えない。

## host namespace ↔ vcluster namespace の対応

| 仮想クラスタ内リソース | ホストクラスタでの実体 |
|---|---|
| `default/my-pod` | `vc-a/my-pod-xxxxx-x-default-x-vc-a` |
| `default/my-svc` | `vc-a/my-svc-xxxxx-x-default-x-vc-a` |
| `kube-system/coredns` | `vc-a/coredns-xxxxx-x-kube-system-x-vc-a` |

名前変換は deterministic（ハッシュベース）なので、ホスト側から Pod を特定できる。

## デプロイ設定

```yaml
# manifests/overlays/vcluster/values-vc-a.yaml
sync:
  toHost:
    ingresses: { enabled: false }
controlPlane:
  service: { spec: { type: ClusterIP } }
```

`ingresses.enabled: false` にすることで Ingress リソースをホストに同期しない（ホストの Ingress controller と競合させない）。

```bash
# vCluster のインストール（Helm）
helm install vc-a vcluster \
  --repo https://charts.loft.sh \
  --namespace vc-a --create-namespace \
  --values manifests/overlays/vcluster/values-vc-a.yaml

# 仮想クラスタに接続
vcluster connect vc-a --namespace vc-a
```

## 用途

### CI 環境

PR ごとに仮想クラスタを作成して E2E テストを実行し、終了後に削除する。
ホストクラスタを汚さずに独立した Kubernetes 環境をすばやく確保できる。

```bash
# PR CI パイプライン例
vcluster create pr-$PR_ID --namespace ci --values ci-values.yaml
kubectl --context pr-$PR_ID apply -f manifests/
# テスト実行
vcluster delete pr-$PR_ID
```

### 教育環境

受講者ごとに独立したクラスタを提供する。受講者は `kubectl` で完全な Kubernetes を体験できる。

### Namespace 超え隔離

標準の Namespace は RBAC で隔離するが、vCluster は API サーバごと分離するためより強い隔離を実現する。
クラスタスコープリソース（CRD / ClusterRole / Node など）の衝突を防げる。

## 類似ツールとの比較

| ツール | アプローチ | 用途 |
|---|---|---|
| **vCluster** | ホスト上に仮想クラスタ | テナント隔離・CI・教育 |
| **Loft** | vCluster のエンタープライズ版 + UI | 企業向けセルフサービス Kubernetes |
| **kcp** | 超軽量 API server（データプレーンなし）| multi-cluster control plane |
| **Crossplane** | Kubernetes でインフラを宣言管理 | クラウドリソースのプロビジョニング |

vCluster はデータプレーン（Pod 実行）もホストに委譲するため、kcp（API server のみ）とは異なりテナントが実際に Pod を動かせる。

## 動作確認

```bash
# 仮想クラスタへの接続
vcluster connect vc-a --namespace vc-a -- kubectl get nodes

# ホスト側で実体を確認
kubectl get pods -n vc-a | head -20

# 仮想クラスタで Pod を作成してホスト側に反映されることを確認
vcluster connect vc-a --namespace vc-a -- \
  kubectl run nginx --image=nginx --restart=Never
kubectl get pods -n vc-a  # 名前変換されて表示される
```
