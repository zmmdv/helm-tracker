package k8s

import (
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "github.com/sirupsen/logrus"
)

func NewClient() (*kubernetes.Clientset, error) {
    log := logrus.StandardLogger()
    
    log.Debug("Creating in-cluster config")
    // Create in-cluster config
    config, err := rest.InClusterConfig()
    if err != nil {
        return nil, err
    }
    log.Debug("In-cluster config created successfully")

    log.Debug("Creating Kubernetes clientset")
    // Create clientset
    clientset, err := kubernetes.NewForConfig(config)
    if err != nil {
        return nil, err
    }
    log.Debug("Kubernetes clientset created successfully")

    return clientset, nil
}