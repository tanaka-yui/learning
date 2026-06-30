# SpinKube — WASM workload・SpinApp CRD・cold-start 計測

## SpinKube とは

**SpinKube** は Fermyon Spin フレームワークのアプリを Kubernetes 上でネイティブに動かすプロジェクトである。
WASM (WebAssembly) モジュールを containerd の shim として直接実行することで、コンテナイメージのオーバーヘッドを排除する。

## アーキテクチャ

```
kubectl apply SpinApp
      │
      ▼
SpinKube operator (spin-operator)
      │  SpinApp → Deployment に変換
      ▼
kubelet → containerd → containerd-shim-spin
                              │
                              ▼
                        wasmtime runtime
                        （.wasm バイナリを直接実行）
```

`containerd-shim-spin` が wasmtime を呼び出し、WASM モジュールを実行する。Linux コンテナが起動しないため cold-start が速い。

## SpinApp CRD

```yaml
# manifests/overlays/spinkube/spinapp.yaml
apiVersion: core.spinkube.dev/v1alpha1
kind: SpinApp
metadata:
  name: demo-spin
  namespace: spinkube-app
spec:
  image: "localhost:5001/demo-spin:v1"
  replicas: 2
  executor: containerd-shim-spin
```

| フィールド | 意味 |
|---|---|
| `image` | OCI レジストリに push した `.wasm` バイナリ（ `wasm32-wasi` ターゲット）|
| `executor` | shim の種類。`containerd-shim-spin` が Fermyon Spin 用 |
| `replicas` | 通常の Deployment と同様 |

## cold-start と footprint 計測

WASM runtime の特徴的な利点はコールドスタートの速さとメモリフットプリントの小ささ。

| 指標 | Linux container (Go) | WASM (Spin) |
|---|---|---|
| cold-start | ~200–500 ms | **~5–20 ms** |
| 常駐メモリ | ~30–50 MB | **~5–10 MB** |
| イメージサイズ | ~20 MB | **~2–5 MB** |

計測は `kubectl run` で Pod を作成してから最初のレスポンスが返るまでの時間で比較できる。

## kind での制限事項

> **重要**: kind ノードは標準の `kindest/node` イメージを使っており、`containerd-shim-spin` がインストールされていない。
> そのため `SpinApp` を apply しても Pod が **Pending のまま**になり、以下のエラーが出る:
>
> ```
> Warning  FailedScheduling  ...  0/3 nodes are available: 3 Insufficient cpu
> ```
> または
> ```
> Error: failed to create containerd task: ... shim not found
> ```
>
> kind での動作確認には以下のいずれかを使う:
> - `kwasm-operator` を使って kind ノードに shim を動的インストールする
> - 事前に `containerd-shim-spin` を組み込んだカスタム `kindest/node` イメージをビルドする
> - Azure AKS / k3s など shim 対応環境で試す

## wasm-on-k8s 採用基準

WASM が適切なユースケース:
- **イベント駆動 workload**: HTTP リクエスト単位で起動する短命処理（Lambda スタイル）
- **polyglot**: Rust / Go / Python / JS など複数言語を同一 runtime で動かしたい
- **高密度マルチテナント**: 1 ノードに数千 WASM module を詰め込む
- **cold-start 制約**: 数十 ms オーダーの初動が必要なリアルタイム処理

WASM が不適切なユースケース:
- 長期接続を保持するステートフルサービス（TCP サーバ等）
- Linux システムコールに直接依存するアプリ
- GPU 計算や特殊デバイスアクセスが必要なワークロード

## KubeVirt との対比

KubeVirt が **Linux VM を Pod として動かす**（隔離強化）のに対し、SpinKube は **WASM モジュールを直接実行する**（軽量化）。目的が逆方向であり、用途が重なることはほぼない。

## 動作確認（kwasm-operator 使用時）

```bash
# kwasm-operator で kind ノードに shim をインストール
helm install kwasm-operator kwasm/kwasm-operator \
  --namespace kwasm --create-namespace \
  --set kwasmOperator.installerImage=ghcr.io/kwasm/kwasm-node-installer:latest

kubectl annotate node --all kwasm.sh/kwasm-node=true

# SpinApp の apply
kubectl apply -k manifests/overlays/spinkube/

# 動作確認
kubectl get spinapp -n spinkube-app
curl http://localhost/spin/hello
```
