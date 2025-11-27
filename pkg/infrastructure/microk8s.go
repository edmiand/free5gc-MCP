package infrastructure

// microk8s.go
// Stubs for microk8s + Helm orchestration. These functions will call out
// to `microk8s` and `helm` (or use client-go) to deploy/remove/scale free5GC.

// TODO: implement using os/exec or Kubernetes client-go with proper permissions.

func DeployFree5GC(chartPath string, releaseName string) error {
	// placeholder
	return nil
}

func RemoveFree5GC(releaseName string) error {
	return nil
}

func ScaleFree5GC(releaseName string, component string, replicas int) error {
	return nil
}

func StatusFree5GC(releaseName string) (string, error) {
	return "unknown", nil
}
