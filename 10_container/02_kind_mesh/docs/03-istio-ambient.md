# Istio アンビエントモード — ztunnel・waypoint・AuthorizationPolicy

## アンビエントモードとは

Istio 1.22 で GA した **アンビエントモード** は、sidecar を Pod に注入せずにサービスメッシュを実現するアーキテクチャである。
従来の sidecar モードと比べて **Pod の改修が不要**・**再起動なしでメッシュ参加**・**footprint が小さい** という特徴を持つ。

## アーキテクチャ: ztunnel + waypoint

アンビエントモードは 2 層で構成される。

### ztunnel (L4 secure overlay)

各ノードに DaemonSet として配置される軽量 proxy。Rust で実装されており、以下を担当する。

- **HBONE (HTTP-Based Overlay Network Environment)** プロトコルでノード間 mTLS トンネルを確立
- L4 ルーティングと認証（接続元 SPIFFE ID の検証）
- Pod ごとの個別注入なしに Namespace 単位でオプトイン

```bash
# アンビエントモードで Namespace をメッシュ参加させる
kubectl label namespace istio-ambient-app istio.io/dataplane-mode=ambient
```

### waypoint proxy (L7)

HTTP ヘッダ操作・retries・timeout・L7 認可などが必要な場合にのみデプロイする Gateway API の `Gateway` リソース。

```yaml
# manifests/overlays/istio-ambient/waypoint.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: waypoint
  annotations:
    istio.io/for-service-account: default
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
```

**Istio 1.30+ の重要な変更**: waypoint を有効にするには Service または ServiceAccount に `istio.io/use-waypoint: <waypoint-name>` ラベルを付与する必要がある。ラベルがないと L7 ポリシーが waypoint を経由せず適用されない。

```bash
# Service に waypoint を紐付ける（Istio 1.30+）
kubectl label service api istio.io/use-waypoint=waypoint -n istio-ambient-app
```

## sidecar モードとの比較

| 観点 | sidecar モード | アンビエントモード |
|---|---|---|
| Pod 改修 | init コンテナ + sidecar 注入が必要 | **不要**（ラベル 1 行）|
| 再起動 | Pod 再起動で注入 | **再起動不要** |
| CPU/メモリ | Pod ごとに Envoy が常駐 | ノード 1 台に ztunnel のみ（L4 のみ）|
| L7 機能 | 全 Pod で利用可能 | waypoint が必要なサービスのみ追加 |
| IPv6 | 対応 | **ztunnel が dual-stack 対応**（sidecar より先行） |
| 運用容易性 | sidecar バージョン管理が複雑 | ztunnel は DaemonSet 単体で更新可能 |

## AuthorizationPolicy の適用順序

アンビエントモードでは `targetRefs` で適用先を指定する。

```yaml
# manifests/overlays/istio-ambient/authpolicy.yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata: { name: deny-admin, namespace: istio-ambient-app }
spec:
  targetRefs: [{ kind: Service, group: "", name: api }]
  action: DENY
  rules:
  - to: [{ operation: { paths: ["/admin/*"] } }]
```

ポリシーは以下の順序で評価される（waypoint 経由の場合）:

1. **ztunnel** — L4 DENY（接続元 SPIFFE ID ベース）
2. **waypoint** — L7 DENY → L7 ALLOW の順に評価
3. **ztunnel (受信側)** — L4 ALLOW の最終確認

## sidecar overlay と同居する場合の label rule

同一クラスタで sidecar モードの Namespace とアンビエントモードの Namespace を混在させることができる。

- `istio-injection: enabled` → sidecar モード
- `istio.io/dataplane-mode: ambient` → アンビエントモード
- **両方付けてはいけない**（istiod が Namespace 単位でどちらか一方を適用する）

sidecar-managed Pod からアンビエント Namespace の Service へのアクセスも ztunnel が中継するため、モード間のトラフィックは透過的に処理される。

## 動作確認

```bash
# waypoint Pod が Running になっていることを確認
kubectl get pods -n istio-ambient-app

# /admin/ パスが 403 になることを確認
kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
  curl -s http://api.istio-ambient-app/admin/secret
```
