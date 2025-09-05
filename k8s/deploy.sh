#!/bin/bash

set -e

echo "Deploying OCPP CSMS to k3s..."

# Apply manifests in order
kubectl apply -f 01-configmap.yaml
kubectl apply -f 02-pvc.yaml
kubectl apply -f 03-deployment.yaml
kubectl apply -f 04-service.yaml

echo "Waiting for deployment to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/ocpp-csms

echo "Deployment status:"
kubectl get pods -l app=ocpp-csms
kubectl get services ocpp-csms-service

echo ""
echo "Service endpoints:"
EXTERNAL_IP=$(kubectl get service ocpp-csms-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
if [ -z "$EXTERNAL_IP" ]; then
  EXTERNAL_IP=$(kubectl get service ocpp-csms-service -o jsonpath='{.spec.clusterIP}')
  echo "Using ClusterIP: $EXTERNAL_IP"
else
  echo "External IP: $EXTERNAL_IP"
fi

echo ""
echo "OCPP WebSocket: ws://$EXTERNAL_IP:8080/ocpp"
echo "Device Manager API: http://$EXTERNAL_IP:8080/device-manager"
