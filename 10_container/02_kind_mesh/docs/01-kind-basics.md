# kind 基礎 — クラスタ構成・local registry・multi-cluster 運用

## kind とは

**kind (Kubernetes IN Docker)** は Docker コンテナをノードとして Kubernetes クラスタをローカルで動かすツールである。
各ノードは `kindest/node` イメージを使ったコンテナであり、その中で systemd・kubelet・containerd が稼働する。
本番環境と同じ API サーバ・CNI・CRI を使うため、マニフェストの互換性が高い。

## cluster.yaml の読み方

```yaml
# kind/cluster.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: learning-base
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - { containerPort: 80,  hostPort: 80,  protocol: TCP }
  - { containerPort: 443, hostPort: 443, protocol: TCP }
- role: worker
- role: worker
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
```

| フィールド | 意味 |
|---|---|
| `extraPortMappings` | Docker の `-p` 相当。`hostPort:80` → コンテナ内 port 80 に転送。Ingress / Gateway のテストに必要 |
| `node-labels: ingress-ready=true` | NGINX Ingress の `nodeSelector` に一致させる慣用設定 |
| `containerdConfigPatches` | `certs.d` ディレクトリを有効化し、後でレジストリミラーを差し込めるようにする |

## local registry mirror

`kind/registry.sh` + `kind/bootstrap.sh` がセットアップする。

1. Docker Hub から `registry:2` コンテナを `kind-registry` 名で起動 (`localhost:5001`)
2. クラスタ作成後、各ノードに `/etc/containerd/certs.d/localhost:5001/hosts.toml` を書き込む

```toml
[host."http://kind-registry:5000"]
```

3. `kind-registry` コンテナを `kind` Docker ネットワークに join させる（ノード→レジストリ間通信を確立）
4. `kube-public/local-registry-hosting` ConfigMap でツールに存在を通知する

これにより `localhost:5001/demo-api:v1` のような image reference が全ノードから pull できる。

## kindnet vs Cilium

kind のデフォルト CNI は **kindnet** (シンプルな ptp + host-local)。L7 NetworkPolicy や eBPF data path は持たない。

Cilium を使う場合は `cluster-cilium.yaml` で `disableDefaultCNI: true` かつ `kubeProxyMode: none` を指定する。

```yaml
# kind/cluster-cilium.yaml
networking:
  disableDefaultCNI: true   # kindnet を無効化
  kubeProxyMode: none        # Cilium が kube-proxy 代替
```

クラスタ起動後に Helm で Cilium をインストールすると CNI と kube-proxy の両方が Cilium に置き換わる。

## multi-cluster 運用

本チャプターでは用途別に 3 クラスタを分離する。

| クラスタ名 | 設定ファイル | 用途 |
|---|---|---|
| `learning-base` | `cluster.yaml` | Istio / SpinKube / Kata / vCluster / Karpenter |
| `learning-cilium` | `cluster-cilium.yaml` | Cilium eBPF CNI・Hubble |
| `learning-linkerd` | `cluster-linkerd.yaml` | Linkerd サービスメッシュ |

複数クラスタを操作するときは `kubectl config use-context kind-<name>` または `--context` フラグで切り替える。
クラスタ作成後は `kind get clusters` で一覧を確認できる。

## 起動・停止の基本フロー

```bash
# 起動（registry も含む）
./kind/bootstrap.sh

# 停止
kind delete cluster --name learning-base
kind delete cluster --name learning-cilium
kind delete cluster --name learning-linkerd
docker stop kind-registry && docker rm kind-registry
```

Makefile の `make up` / `make down` がこれらをラップしている。
