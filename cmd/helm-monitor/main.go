package main

import (
    "os"
    "os/signal"
    "syscall"

    "github.com/sirupsen/logrus"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "helm-monitor/pkg/helm"
)

func main() {
    log := logrus.New()
    log.Debug("Starting helm-monitor application")

    // Initialize Kubernetes client
    log.Debug("Initializing Kubernetes client")
    config, err := rest.InClusterConfig()
    if err != nil {
        log.Fatalf("Failed to get in-cluster config: %v", err)
    }

    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        log.Fatalf("Failed to create Kubernetes client: %v", err)
    }
    log.Debug("Kubernetes client initialized successfully")

    // Initialize Helm monitor
    log.Debug("Initializing Helm monitor")
    monitor := helm.NewMonitor(clientset, log)
    log.Debug("Helm monitor initialized successfully")

    // Get check interval from environment
    intervalStr := os.Getenv("CHECK_INTERVAL")
    if intervalStr == "" {
        intervalStr = "6h"
    }
    log.Debugf("Starting monitoring loop with interval: %s", intervalStr)

    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Start the monitor
    go monitor.Start()

    // Wait for shutdown signal
    <-sigChan
    log.Info("Received shutdown signal, exiting...")
}