#!/bin/bash

# 1. クラスター管理者として認証されていることを確認
kubectl auth can-i create secrets --namespace kube-system
if [ $? -ne 0 ]; then
    echo "Error: You don't have sufficient permissions to create secrets in the kube-system namespace."
    exit 1
fi

# 2. ブートストラップトークンの作成
TOKEN_ID=$(openssl rand -hex 3)
TOKEN_SECRET=$(openssl rand -hex 8)
BOOTSTRAP_TOKEN="${TOKEN_ID}.${TOKEN_SECRET}"

# 3. トークンを含むシークレットの作成
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: bootstrap-token-${TOKEN_ID}
  namespace: kube-system
type: bootstrap.kubernetes.io/token
stringData:
  token-id: ${TOKEN_ID}
  token-secret: ${TOKEN_SECRET}
  usage-bootstrap-authentication: "true"
  usage-bootstrap-signing: "true"
  auth-extra-groups: system:bootstrappers:worker,system:bootstrappers:ingress
EOF

# 4. トークンの有効期限設定（24時間後）
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    EXPIRATION=$(date -v+24H -u +"%Y-%m-%dT%H:%M:%SZ")
else
    # Linux
    EXPIRATION=$(date -d "+24 hours" -u +"%Y-%m-%dT%H:%M:%SZ")
fi

# base64エンコードされた有効期限を作成
EXPIRATION_BASE64=$(echo -n "$EXPIRATION" | base64)

# secretをパッチ
kubectl patch secret bootstrap-token-${TOKEN_ID} -n kube-system --type='json' -p='[{"op": "add", "path": "/data/expiration", "value":"'$EXPIRATION_BASE64'"}]'

# 5. クラスターロールバインディングの作成（自動承認用）
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: auto-approve-csrs-for-group
subjects:
- kind: Group
  name: system:bootstrappers
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
  apiGroup: rbac.authorization.k8s.io
EOF

cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: auto-approve-renewals-for-nodes
subjects:
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
  apiGroup: rbac.authorization.k8s.io
EOF

cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: create-csrs-for-bootstrapping
subjects:
- kind: Group
  name: system:bootstrappers
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:node-bootstrapper
  apiGroup: rbac.authorization.k8s.io
EOF

# 6. 作成したトークンの表示
echo "Bootstrap Token: ${BOOTSTRAP_TOKEN}"
echo "Use this token in your node configuration."