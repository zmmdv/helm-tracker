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
• monitoring-loki in namespace monitoring: 6.22.0 → 6.25.0
• monitoring-prometheus-read in namespace monitoring: 26.1.0 → 27.1.0
• monitoring-tempo in namespace monitoring: 1.14.0 → 1.18.1
• nats in namespace nats: 8.3.0 → 9.0.1
• redis-cluster in namespace redis: 11.0.3 → 11.4.1
Next notification will be sent after: 2024-01-20 12:00:00
```

![image](https://github.com/user-attachments/assets/dea6f9de-fc20-4649-b90d-699a786f5188)


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
