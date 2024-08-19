#!/bin/bash

# kubeconfigファイルのパス
KUBECONFIG_PATH="${1:-$KUBECONFIG}"

if [ -z "$KUBECONFIG_PATH" ]; then
    echo "Error: KUBECONFIG path not specified. Please provide the path as an argument or set the KUBECONFIG environment variable."
    exit 1
fi

if [ ! -f "$KUBECONFIG_PATH" ]; then
    echo "Error: KUBECONFIG file not found at $KUBECONFIG_PATH"
    exit 1
fi

# CA証明書の抽出
CA_DATA=$(awk '/certificate-authority-data:/ {print $2; exit}' "$KUBECONFIG_PATH")

if [ -z "$CA_DATA" ]; then
    echo "Error: certificate-authority-data not found in the kubeconfig file."
    exit 1
fi

echo "$CA_DATA" | base64 -d > ca.crt

if [ $? -ne 0 ]; then
    echo "Error: Failed to decode the CA certificate data."
    exit 1
fi

echo "CA certificate successfully extracted to ca.crt"