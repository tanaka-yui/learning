#!/bin/sh
set -e
./kind/registry.sh
kind create cluster --config kind/cluster.yaml || true
# containerd に registry mirror 設定を撒く
for node in $(kind get nodes --name learning-base); do
  docker exec "$node" mkdir -p /etc/containerd/certs.d/localhost:5001
  cat <<EOF | docker exec -i "$node" tee /etc/containerd/certs.d/localhost:5001/hosts.toml >/dev/null
[host."http://kind-registry:5000"]
EOF
done
# kind network に registry を join
docker network connect "kind" kind-registry 2>/dev/null || true
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:5001"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF
