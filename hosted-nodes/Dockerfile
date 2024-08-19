FROM ubuntu:22.04

# 環境変数の設定
ENV DEBIAN_FRONTEND=noninteractive
ENV KUBERNETES_VERSION=1.28.3

# システムの更新と必要なパッケージのインストール
RUN apt-get update && apt-get install -y \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    iptables \
    software-properties-common \
    && rm -rf /var/lib/apt/lists/*

# Kubernetes リポジトリの追加
RUN curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.28/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg && \
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.28/deb/ /" > /etc/apt/sources.list.d/kubernetes.list

# kubelet、kubeadm、kubectlのインストール
RUN apt-get update && apt-get install -y \
    kubelet=${KUBERNETES_VERSION}-* \
    kubeadm=${KUBERNETES_VERSION}-* \
    kubectl=${KUBERNETES_VERSION}-* \
    && apt-mark hold kubelet kubeadm kubectl

# Dockerリポジトリの追加とcontainerdのインストール
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && apt-get install -y containerd.io

# Modify containerd configuration
RUN mkdir -p /etc/containerd && containerd config default > /etc/containerd/config.toml && \
    sed -i 's/SystemdCgroup = true/SystemdCgroup = false/' /etc/containerd/config.toml && \
    sed -i 's/snapshotter = "overlayfs"/snapshotter = "native"/' /etc/containerd/config.toml

 # Install CNI plugins
 RUN mkdir -p /opt/cni/bin && \
    curl -L "https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz" | tar -C /opt/cni/bin -xz

# Install Calico CNI plugins
RUN mkdir -p /opt/cni/bin && \
    curl -L https://github.com/projectcalico/cni-plugin/releases/download/v3.20.6/calico-amd64 -o /opt/cni/bin/calico && \
    curl -L https://github.com/projectcalico/cni-plugin/releases/download/v3.20.6/calico-ipam-amd64 -o /opt/cni/bin/calico-ipam && \
    chmod +x /opt/cni/bin/calico /opt/cni/bin/calico-ipam

# Add basic CNI configuration for Calico
RUN mkdir -p /etc/cni/net.d
COPY 10-calico.conflist /etc/cni/net.d/10-calico.conflist

# Create necessary directories for Calico
RUN mkdir -p /var/lib/calico && \
    mkdir -p /var/run/calico

# kubeletの設定
RUN mkdir -p /var/lib/kubelet && \
    echo "KUBELET_EXTRA_ARGS=--container-runtime=remote --container-runtime-endpoint=unix:///run/containerd/containerd.sock" > /etc/default/kubelet

# CA証明書のコピー
COPY ca.crt /etc/kubernetes/pki/ca.crt

# 起動スクリプトの追加
COPY start-kubelet.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/start-kubelet.sh

# 必要なディレクトリとファイルの作成
RUN mkdir -p /var/log/journal /var/lib/kubelet /etc/kubernetes/manifests /run/containerd && \
    touch /dev/kmsg

# コンテナ起動時に実行するコマンド
CMD ["/usr/local/bin/start-kubelet.sh"]
