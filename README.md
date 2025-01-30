# Helm Monitor

A Kubernetes application that monitors Helm releases in your cluster and compares them with the latest versions available in their respective repositories. It can notify you through Slack when updates are available.

## Features

- Monitors Helm releases across all namespaces
- Compares installed versions with latest available versions in configured repositories
- Supports flexible checking intervals (minutes, hours, days, or weekly schedules)
- Slack notifications for available updates
- Configurable through YAML
- Memory-efficient batch processing
- Kubernetes-native deployment

## Prerequisites

- Kubernetes cluster
- Helm v3
- Slack workspace (if notifications are enabled)

## Configuration

### Environment Variables

- `CHECK_INTERVAL`: Time between checks (default: "6h")
  - Supports formats: "1m", "1h", "1d", "1w", "1w/monday"
- `LOG_LEVEL`: Logging level (default: "info")
  - Supported values: "debug", "info", "warn", "error"
- `SLACK_CHANNEL_ID`: Slack channel ID for notifications
- `SLACK_BOT_TOKEN`: Slack bot token for authentication

### Repository Configuration

Create a ConfigMap with your repository configuration:

## Deployment

1. Apply the all-in-one deployment file:

```bash
kubectl apply -f deployment/all-in-one.yml
```

This will create:
- ServiceAccount with required permissions
- ClusterRole and ClusterRoleBinding
- Deployment
- ConfigMap for repository configuration

2. Configure Slack notifications (optional):
   - Create a Slack App in your workspace
   - Add the bot to your desired channel
   - Set the `SLACK_CHANNEL_ID` and `SLACK_BOT_TOKEN` environment variables

## Resource Requirements

Default resource limits:

## Security

The application runs with:
- Non-root user (UID 1000)
- Non-privileged container
- Read-only permissions on Kubernetes resources
- Limited RBAC permissions

## Example Configuration

## Slack Notifications

When updates are available, the application sends formatted messages to Slack:

```
[HELM-MONITOR] Helm Chart Updates Available:
•    release: application-keto-admin
      namespace: application
      installed: 0.50.6
      latest in remote repo: 0.50.7
•    release: application-kratos-admin
      namespace: application
      installed: 0.50.6
      latest in remote repo: 0.50.7
•    release: aws-efs-csi-driver-production
      namespace: kube-system
      installed: 2.4.8
      latest in remote repo: 3.1.5
•    release: aws-load-balancer-controller-production
      namespace: kube-system
      installed: 1.8.1
      latest in remote repo: 1.11.0
Next notification will be sent after: UTC 2025-02-06 20:33:00
```

![image](https://github.com/user-attachments/assets/8a1247f4-f40a-4317-ab82-a66e2f0940b1)

## Building from Source

```bash
# Build the Docker image
docker build -t helm-monitor .
```

## License

MIT License

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Support

If you encounter any problems or have suggestions, please open an issue in the GitHub repository.
