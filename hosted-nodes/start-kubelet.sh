#!/bin/bash

# 環境変数の設定
API_SERVER="https://3.114.228.201:6443"
TOKEN="c3ef33.0bef620c5b8cd66a"  # 実際のトークンに置き換える必要があります

# /dev/kmsg のエミュレーション
if [ ! -e /dev/kmsg ]; then
    mknod /dev/kmsg c 1 11
fi

# 必要なディレクトリの作成と権限設定
mkdir -p /var/lib/kubelet /etc/kubernetes/pki
chmod 711 /run/containerd

# kubelet設定ファイルの作成
cat <<EOF >/var/lib/kubelet/config.yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
  x509:
    clientCAFile: "/etc/kubernetes/pki/ca.crt"
authorization:
  mode: Webhook
clusterDomain: "cluster.local"
clusterDNS:
  - "10.96.0.10"
runtimeRequestTimeout: "15m"
failSwapOn: false
EOF

# kubeconfig ファイルの作成
cat <<EOF >/etc/kubernetes/kubelet.conf
apiVersion: v1
clusters:
- cluster:
    certificate-authority: /etc/kubernetes/pki/ca.crt
    server: ${API_SERVER}
  name: default-cluster
contexts:
- context:
    cluster: default-cluster
    namespace: default
    user: default-auth
  name: default-context
current-context: default-context
kind: Config
preferences: {}
users:
- name: default-auth
  user:
    token: ${TOKEN}
EOF

# bootstrap-kubelet.conf ファイルの作成 (トークンを使用)
cat <<EOF >/etc/kubernetes/bootstrap-kubelet.conf
apiVersion: v1
clusters:
- cluster:
    certificate-authority: /etc/kubernetes/pki/ca.crt
    server: ${API_SERVER}
  name: default-cluster
contexts:
- context:
    cluster: default-cluster
    user: kubelet-bootstrap
  name: default-context
current-context: default-context
kind: Config
preferences: {}
users:
- name: kubelet-bootstrap
  user:
    token: ${TOKEN}
EOF

# containerdの起動
containerd &

# containerdが起動するまで少し待つ
sleep 5

# kubeletのバージョンを確認
KUBELET_VERSION=$(/usr/bin/kubelet --version | awk '{print $2}')

# Generate a unique node name
# NODE_NAME="hn-$(hostname | md5sum | cut -c1-6)"
NODE_NAME="hn-x"

# Create Calico nodename file
echo "${NODE_NAME}" > /var/lib/calico/nodename

# Function to check if node is registered
check_node_registered() {
  kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node ${NODE_NAME} &> /dev/null
}

# kubeletの起動（フォアグラウンドで実行）
exec /usr/bin/kubelet \
  --config=/var/lib/kubelet/config.yaml \
  --container-runtime-endpoint=unix:///run/containerd/containerd.sock \
  --kubeconfig=/etc/kubernetes/kubelet.conf \
  --bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf \
  --node-ip=$(hostname -I | awk '{print $1}') \
  --hostname-override=${NODE_NAME} \
  --v=2 \
  --fail-swap-on=false &

# Wait for node to register
echo "Waiting for node to register..."
for i in {1..30}; do
  if check_node_registered; then
    echo "Node registered successfully"
    break
  fi
  echo "Waiting for node registration... attempt $i"
  sleep 10
done

if ! check_node_registered; then
  echo "Failed to register node after multiple attempts"
  exit 1
fi

# Keep the script running
wait
