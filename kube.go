//go:build kube

// This file is excluded from the default build by the `kube` build tag, so the
// application binary never grows a Kubernetes dependency it does not need.
//
// It exists so that k8s.io/client-go — and the k8s.io/api and
// k8s.io/apimachinery graphs it drags in behind it — are real requirements in
// go.mod. Build tags do not affect module resolution, so these modules are
// still downloaded by `go mod download` and still count against cold cache
// time. Same trick as integration_test.go and testcontainers.
//
// Compile it deliberately with:
//
//	go build -tags kube ./...
package main

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// countPods builds a clientset from the ambient kubeconfig and counts the pods
// in a namespace. Nothing calls it; it is here to make the import real.
func countPods(ctx context.Context, kubeconfig, namespace string) (int, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return 0, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return 0, err
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(pods.Items), nil
}
