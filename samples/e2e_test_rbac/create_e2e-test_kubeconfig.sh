#!/bin/sh

set -x
# 首先需要把设置好读写证书，如 export KUBECONFIG=kube-write.config

kubectl apply -f service_account.yaml
kubectl apply -f service_account_cluster_role.yaml
kubectl apply -f service_account_cluster_role_binding.yaml
kubectl apply -f secret.yaml

# 请根据您的环境和需求进行调整
NAMESPACE="default"        # 指定命名空间
SERVICE_ACCOUNT_NAME="e2e-test" # 替换为您的 ServiceAccount 名称
SECRET_NAME="$SERVICE_ACCOUNT_NAME-secret"

# 获取 Secret 的 CA 证书和 Token
CA_CERT=$(kubectl get secret $SECRET_NAME -n $NAMESPACE -o=jsonpath='{.data.ca\.crt}' | base64 --decode)
TOKEN=$(kubectl get secret $SECRET_NAME -n $NAMESPACE -o=jsonpath='{.data.token}' | base64 --decode)

# 获取集群信息
CLUSTER_NAME=$(kubectl config current-context) # 获取当前上下文名称
CLUSTER_ENDPOINT=$(kubectl cluster-info | grep 'Kubernetes control plane' | awk '/https/ {print $NF}')

# 生成 kubeconfig 文件
KUBECONFIG_FILE="kubeconfig-${SERVICE_ACCOUNT_NAME}.yaml"

cat <<EOF > $KUBECONFIG_FILE
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: $(echo "$CA_CERT" | base64)
    server: $CLUSTER_ENDPOINT
  name: $CLUSTER_NAME
contexts:
- context:
    cluster: $CLUSTER_NAME
    user: $SERVICE_ACCOUNT_NAME
  name: ${SERVICE_ACCOUNT_NAME}-context
current-context: ${SERVICE_ACCOUNT_NAME}-context
users:
- name: $SERVICE_ACCOUNT_NAME
  user:
    token: $TOKEN
EOF

echo "kubeconfig 文件已生成: $KUBECONFIG_FILE"
