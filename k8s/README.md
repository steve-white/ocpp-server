# OCPP CSMS Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the OCPP CSMS application to a k3s cluster.

## Files

- `01-configmap.yaml` - Application configuration
- `02-pvc.yaml` - Persistent volume claim for SQLite database
- `03-deployment.yaml` - Main application deployment
- `04-service.yaml` - Service to expose the application
- `deploy.sh` - Deployment script
- `cleanup.sh` - Cleanup script

## Prerequisites

- k3s cluster running on remote VM
- Docker image built and available locally or in registry
- kubectl configured to access the k3s cluster

## Image Deployment Options

### Option 1: Transfer Image via SCP (Recommended for local development)

```bash
# Build the image locally
docker build -t ocpp-csms:latest .

# Save the image to a tar file
docker save ocpp-csms:latest > ocpp-csms.tar

# Copy to remote VM
scp ocpp-csms.tar user@remote-vm:/tmp/

# On the remote VM, load the image
ssh user@remote-vm "sudo k3s ctr images import /tmp/ocpp-csms.tar"
```

### Option 2: Use Container Registry

If using a container registry, update the image reference in `03-deployment.yaml`:

```yaml
image: your-registry/ocpp-csms:latest
imagePullPolicy: Always
```

## Deployment

1. **Copy manifests to remote VM:**
   ```bash
   scp -r k8s/ user@remote-vm:/tmp/
   ```

2. **Deploy the application:**
   ```bash
   ssh user@remote-vm "cd /tmp/k8s && chmod +x deploy.sh && ./deploy.sh"
   ```

3. **Check deployment status:**
   ```bash
   ssh user@remote-vm "kubectl get all -l app=ocpp-csms"
   ```

4. **Get service endpoint:**
   ```bash
   ssh user@remote-vm "kubectl get service ocpp-csms-service"
   ```

## Accessing the Application

The application will be available at:

- **OCPP WebSocket**: `ws://<EXTERNAL-IP>:8080/ocpp`
- **Device Manager API**: `http://<EXTERNAL-IP>:8080/device-manager`

Where `<EXTERNAL-IP>` is the external IP of the LoadBalancer service.

## Configuration

The application configuration is stored in the ConfigMap (`01-configmap.yaml`). Modify this file to change application settings before deployment.

Key configuration sections:
- `csms_server` - WebSocket server configuration
- `device_manager` - REST API configuration
- `session` - Transaction handling configuration
- `message_manager` - Message processing configuration

## Persistence

- SQLite database is persisted using a PersistentVolumeClaim
- Logs are stored in an emptyDir volume (ephemeral)
- Application configuration is mounted from ConfigMap

## Cleanup

To remove the deployment:

```bash
ssh user@remote-vm "cd /tmp/k8s && ./cleanup.sh"
```

## Troubleshooting

1. **Check pod logs:**
   ```bash
   kubectl logs deployment/ocpp-csms
   ```

2. **Check pod status:**
   ```bash
   kubectl describe pod -l app=ocpp-csms
   ```

3. **Check service:**
   ```bash
   kubectl describe service ocpp-csms-service
   ```

4. **Port forward for local testing:**
   ```bash
   kubectl port-forward service/ocpp-csms-service 8080:8080
   ```

## Notes

- The deployment uses `imagePullPolicy: Never` by default, assuming local image import
- WebSocket connections use session affinity to ensure proper connection handling
- The application runs as a single replica due to SQLite database constraints
- Health checks are configured to ensure proper startup and running state
