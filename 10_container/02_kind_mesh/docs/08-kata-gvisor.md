# Kata / gVisor — RuntimeClass・サンドボックスランタイム・性能トレードオフ

## サンドボックスランタイムとは

標準の runc コンテナはホスト Linux カーネルを共有するため、コンテナエスケープ脆弱性のリスクがある。
**サンドボックスランタイム** はコンテナとホストカーネルの間に追加の隔離層を挿入する。

| ランタイム | 隔離手段 | 特徴 |
|---|---|---|
| runc (標準) | cgroup + namespace | 軽量・高速・カーネル共有 |
| **Kata Containers** | 軽量 VM (QEMU/Cloud Hypervisor) | VM レベル隔離・性能オーバーヘッドあり |
| **gVisor** | user-space カーネル (Go) | syscall インターセプト・ホストカーネル非共有 |

## RuntimeClass

Kubernetes の `RuntimeClass` リソースで Pod ごとに使うランタイムを指定できる。

```yaml
# RuntimeClass の定義（kata-deploy が自動作成）
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: kata
handler: kata          # CRI に渡すハンドラ名
```

Pod 側での指定:

```yaml
# manifests/overlays/kata/api-kata.yaml（抜粋）
spec:
  runtimeClassName: kata   # ← 指定するだけ
  containers:
  - name: api
    image: localhost:5001/demo-api:v1
```

> **Kubernetes 1.33+ の重要な変更**: RuntimeClass の admission controller が強化された。
> 存在しない `runtimeClassName` を指定した Pod は **Pending にならずに REJECT（作成失敗）** する。
> `kubectl apply` 時点でエラーが返るため、RuntimeClass を事前にインストールしておく必要がある。
> 旧バージョンは Pending になるだけだったが、1.33 以降は apply 自体が失敗する。

## Kata Containers

Kata は Pod ごとに **軽量 VM** を起動し、その VM 内で標準のコンテナ（runc）を動かす。

```
Pod
 └── kata-agent (ゲスト VM 内)
       └── runc コンテナ
       └── ゲスト Linux カーネル  ← ホストカーネルと分離
 └── kata-shim (ホスト側)
       └── QEMU / Cloud Hypervisor / Firecracker
```

VM の起動に数百 ms かかるが、ホストカーネルを完全に分離できる。
multi-tenant 環境（複数顧客の workload が同居する SaaS）で採用される。

## gVisor

gVisor は **user-space に Linux カーネルを Go で再実装** した `runsc` ランタイムである。

```
Pod → runsc (Sentry)
        │  syscall を User-space で処理
        └── Gofer (ファイルシステムアクセスのみホストに委譲)
```

VM を使わないため Kata より起動が速いが、syscall ごとにコンテキストスイッチが発生する。
未実装 syscall があるアプリは動作しないケースがある。

## kind で動かない理由

kind ノード（Docker コンテナ）はネストされた仮想化環境であり:

- **Kata**: Docker コンテナ内で QEMU を動かすには nested virtualization が必要（デフォルト無効）
- **gVisor**: `runsc` は `/proc/self/exe` の置換など Linux 固有の機構に依存。Docker コンテナ内では制約がある

kind で Kata/gVisor を試す場合は `--privileged` フラグや nested virtualization 有効化が必要で、学習環境としては通常 **AWS Bare Metal インスタンス / GKE / AKS** で確認する。

## 本物環境への持ち出し

```bash
# GKE の場合（Sandbox 対応ノードプール）
gcloud container node-pools create sandbox-pool \
  --cluster my-cluster \
  --sandbox type=gvisor

# AKS の場合（Kata がプレビュー）
az aks nodepool add \
  --cluster-name my-cluster \
  --name katapool \
  --workload-runtime KataMshvVmIsolation
```

## 性能トレードオフ

| 指標 | runc | gVisor | Kata |
|---|---|---|---|
| syscall レイテンシ | ベースライン | 2–5x | 2–3x |
| 起動時間 | ~50 ms | ~100 ms | ~500 ms–1 s |
| メモリオーバーヘッド | なし | ~15 MB (Sentry) | ~64–256 MB (VM) |
| カーネル隔離 | なし | **user-space カーネル** | **完全 VM 隔離** |
| 未実装 syscall | なし | 一部あり | ほぼなし |

マルチテナント要件が強いほど隔離強度の高い Kata を選び、パフォーマンスが重要なら gVisor か標準 runc を選ぶ。

## 動作確認（bare metal / nested virt 有効環境）

```bash
# Kata RuntimeClass のインストール
kubectl apply -f https://raw.githubusercontent.com/kata-containers/kata-containers/main/tools/packaging/kata-deploy/kata-rbac/base/kata-rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/kata-containers/kata-containers/main/tools/packaging/kata-deploy/kata-deploy/base/kata-deploy.yaml

# Kata Pod の起動
kubectl apply -k manifests/overlays/kata/

# VM で動いていることを確認（uname -r が異なる）
kubectl exec -n kata-app deploy/api-kata -- uname -r
```
