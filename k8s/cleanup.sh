#!/bin/bash

echo "Cleaning up OCPP CSMS deployment..."

kubectl delete -f 04-service.yaml --ignore-not-found=true
kubectl delete -f 03-deployment.yaml --ignore-not-found=true
kubectl delete -f 02-pvc.yaml --ignore-not-found=true
kubectl delete -f 01-configmap.yaml --ignore-not-found=true

echo "Cleanup complete."
