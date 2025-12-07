package k8s

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Manager handles Kubernetes and Helm operations
type Manager struct {
	k8sTool            string // "microk8s" | "kubectl" | "k3s"
	HelmBasePath       string
	chartPath          string
	ueransimChartPath  string
	namespace          string
	releaseName        string
}

// NewManager creates a new K8s manager
func NewManager(k8sTool, helmBasePath, chartPath, ueransimChartPath, namespace, releaseName string) *Manager {
	return &Manager{
		k8sTool:           k8sTool,
		HelmBasePath:      helmBasePath,
		chartPath:         chartPath,
		ueransimChartPath: ueransimChartPath,
		namespace:         namespace,
		releaseName:       releaseName,
	}
}

// getKubectlCommand returns the appropriate kubectl command based on k8s tool
func (m *Manager) getKubectlCommand() string {
	switch m.k8sTool {
	case "microk8s":
		return "microk8s kubectl"
	case "k3s":
		return "k3s kubectl"
	default:
		return "kubectl"
	}
}

// getHelmCommand returns the appropriate helm command based on k8s tool
func (m *Manager) getHelmCommand() string {
	switch m.k8sTool {
	case "microk8s":
		return "microk8s helm"
	case "k3s":
		return "k3s helm"
	default:
		return "helm"
	}
}

// CheckK8sStatus checks if the k8s tool is running and healthy
func (m *Manager) CheckK8sStatus() error {
	var cmd *exec.Cmd

	switch m.k8sTool {
	case "microk8s":
		cmd = exec.Command("microk8s", "status", "--wait-ready", "--timeout", "10")
	case "k3s":
		cmd = exec.Command("systemctl", "is-active", "k3s")
	default:
		// For standard kubectl, check if we can get cluster info
		kubectlParts := strings.Fields(m.getKubectlCommand())
		kubectlParts = append(kubectlParts, "cluster-info")
		cmd = exec.Command(kubectlParts[0], kubectlParts[1:]...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("k8s tool %s is not ready: %v, output: %s", m.k8sTool, err, string(output))
	}

	return nil
}

// StartK8sTool starts the k8s tool if it's not running
func (m *Manager) StartK8sTool() error {
	var cmd *exec.Cmd

	switch m.k8sTool {
	case "microk8s":
		cmd = exec.Command("microk8s", "start")
	case "k3s":
		cmd = exec.Command("systemctl", "start", "k3s")
	default:
		return fmt.Errorf("cannot auto-start k8s tool: %s", m.k8sTool)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start k8s tool %s: %v, output: %s", m.k8sTool, err, string(output))
	}

	// Wait a bit for k8s to be ready
	time.Sleep(5 * time.Second)

	// Verify it started successfully
	return m.CheckK8sStatus()
}

// EnsureK8sRunning ensures k8s is running, starts it if needed
func (m *Manager) EnsureK8sRunning() error {
	if err := m.CheckK8sStatus(); err != nil {
		fmt.Printf("K8s not ready, attempting to start %s...\n", m.k8sTool)
		if err := m.StartK8sTool(); err != nil {
			return fmt.Errorf("failed to ensure k8s is running: %v", err)
		}
		fmt.Printf("Successfully started %s\n", m.k8sTool)
	}
	return nil
}

// CreateNamespace creates the namespace if it doesn't exist
func (m *Manager) CreateNamespace() error {
	kubectlParts := strings.Fields(m.getKubectlCommand())
	kubectlParts = append(kubectlParts, "create", "namespace", m.namespace)
	cmd := exec.Command(kubectlParts[0], kubectlParts[1:]...)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore error if namespace already exists
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create namespace: %v, output: %s", err, string(output))
	}
	return nil
}

// StartFree5GCHelm installs or upgrades free5gc using helm
func (m *Manager) StartFree5GCHelm() error {
	// Ensure k8s is running
	if err := m.EnsureK8sRunning(); err != nil {
		return err
	}

	// Create namespace if needed
	if err := m.CreateNamespace(); err != nil {
		return err
	}

	// Install or upgrade using helm
	helmParts := strings.Fields(m.getHelmCommand())
	helmParts = append(helmParts, "upgrade", "--install",
		m.releaseName,
		m.chartPath,
		"-n", m.namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m")

	cmd := exec.Command(helmParts[0], helmParts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install/upgrade free5gc helm chart: %v, output: %s", err, string(output))
	}

	return nil
}

// StopFree5GCHelm uninstalls free5gc helm release
func (m *Manager) StopFree5GCHelm() error {
	// Check if release exists
	helmParts := strings.Fields(m.getHelmCommand())
	checkParts := append(helmParts, "list", "-n", m.namespace, "-q")
	checkCmd := exec.Command(checkParts[0], checkParts[1:]...)
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check helm releases: %v", err)
	}

	// If release doesn't exist, nothing to do
	if !strings.Contains(string(checkOutput), m.releaseName) {
		return fmt.Errorf("helm release %s not found in namespace %s", m.releaseName, m.namespace)
	}

	// Uninstall the release
	uninstallParts := append(helmParts, "uninstall", m.releaseName, "-n", m.namespace, "--wait")
	cmd := exec.Command(uninstallParts[0], uninstallParts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall free5gc helm chart: %v, output: %s", err, string(output))
	}

	// Wait a bit for pods to terminate
	time.Sleep(3 * time.Second)

	// Verify all free5gc pods are gone
	kubectlParts := strings.Fields(m.getKubectlCommand())
	verifyParts := append(kubectlParts, "get", "pods", "-n", m.namespace, "--no-headers")
	verifyCmd := exec.Command(verifyParts[0], verifyParts[1:]...)
	verifyOutput, _ := verifyCmd.CombinedOutput()

	if strings.TrimSpace(string(verifyOutput)) != "" {
		// There are still some pods, but this might be expected during termination
		fmt.Printf("Note: Some pods may still be terminating: %s\n", string(verifyOutput))
	}

	return nil
}

// GetFree5GCHelmStatus returns the status of free5gc pods
func (m *Manager) GetFree5GCHelmStatus() (string, error) {
	kubectlParts := strings.Fields(m.getKubectlCommand())
	kubectlParts = append(kubectlParts, "get", "pods", "-n", m.namespace)
	cmd := exec.Command(kubectlParts[0], kubectlParts[1:]...)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get pod status: %v, output: %s", err, string(output))
	}

	return string(output), nil
}

// UpgradeFree5GCHelm upgrades the existing free5gc helm deployment
// Uses uninstall + install strategy to avoid pending issues with configuration changes
func (m *Manager) UpgradeFree5GCHelm() error {
	// Ensure k8s is running
	if err := m.EnsureK8sRunning(); err != nil {
		return err
	}

	// Check if release exists
	helmParts := strings.Fields(m.getHelmCommand())
	checkParts := append(helmParts, "list", "-n", m.namespace, "-q")
	checkCmd := exec.Command(checkParts[0], checkParts[1:]...)
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check helm releases: %v", err)
	}

	// If release doesn't exist, return error
	if !strings.Contains(string(checkOutput), m.releaseName) {
		return fmt.Errorf("helm release %s not found in namespace %s. Please start free5gc first using k8s_start_free5gc", m.releaseName, m.namespace)
	}

	fmt.Printf("Uninstalling existing free5gc release...\n")
	
	// Uninstall the existing release
	uninstallParts := append(helmParts, "uninstall", m.releaseName, "-n", m.namespace, "--wait")
	uninstallCmd := exec.Command(uninstallParts[0], uninstallParts[1:]...)
	uninstallOutput, err := uninstallCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall free5gc helm chart: %v, output: %s", err, string(uninstallOutput))
	}

	// Wait for pods to fully terminate
	fmt.Printf("Waiting for pods to terminate...\n")
	time.Sleep(5 * time.Second)

	// Verify pods are gone
	kubectlParts := strings.Fields(m.getKubectlCommand())
	for i := 0; i < 30; i++ {
		verifyParts := append(kubectlParts, "get", "pods", "-n", m.namespace, "--no-headers")
		verifyCmd := exec.Command(verifyParts[0], verifyParts[1:]...)
		verifyOutput, _ := verifyCmd.CombinedOutput()
		
		if strings.TrimSpace(string(verifyOutput)) == "" {
			break
		}
		
		if i == 29 {
			fmt.Printf("Warning: Some pods still exist, proceeding anyway: %s\n", string(verifyOutput))
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("Installing free5gc with new configuration...\n")

	// Install with new configuration
	installParts := append(helmParts, "install",
		m.releaseName,
		m.chartPath,
		"-n", m.namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m")

	installCmd := exec.Command(installParts[0], installParts[1:]...)
	installOutput, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install free5gc helm chart: %v, output: %s", err, string(installOutput))
	}

	fmt.Printf("Successfully upgraded free5gc\n")
	return nil
}

// StartUeransimHelm installs or upgrades ueransim using helm
func (m *Manager) StartUeransimHelm() error {
	// Ensure k8s is running
	if err := m.EnsureK8sRunning(); err != nil {
		return err
	}

	// Create namespace if needed
	if err := m.CreateNamespace(); err != nil {
		return err
	}

	if m.ueransimChartPath == "" {
		return fmt.Errorf("ueransim chart path not configured")
	}

	// Install or upgrade using helm
	helmParts := strings.Fields(m.getHelmCommand())
	helmParts = append(helmParts, "upgrade", "--install",
		"ueransim",
		m.ueransimChartPath,
		"-n", m.namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m")

	cmd := exec.Command(helmParts[0], helmParts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install/upgrade ueransim helm chart: %v, output: %s", err, string(output))
	}

	return nil
}

// StopUeransimHelm uninstalls ueransim helm release
func (m *Manager) StopUeransimHelm() error {
	// Check if release exists
	helmParts := strings.Fields(m.getHelmCommand())
	checkParts := append(helmParts, "list", "-n", m.namespace, "-q")
	checkCmd := exec.Command(checkParts[0], checkParts[1:]...)
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check helm releases: %v", err)
	}

	// If release doesn't exist, nothing to do
	if !strings.Contains(string(checkOutput), "ueransim") {
		return fmt.Errorf("helm release ueransim not found in namespace %s", m.namespace)
	}

	// Uninstall the release
	uninstallParts := append(helmParts, "uninstall", "ueransim", "-n", m.namespace, "--wait")
	cmd := exec.Command(uninstallParts[0], uninstallParts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to uninstall ueransim helm chart: %v, output: %s", err, string(output))
	}

	// Wait a bit for pods to terminate
	time.Sleep(3 * time.Second)

	return nil
}

// GetUeransimHelmStatus returns the status of ueransim pods
func (m *Manager) GetUeransimHelmStatus() (string, error) {
	kubectlParts := strings.Fields(m.getKubectlCommand())
	kubectlParts = append(kubectlParts, "get", "pods", "-n", m.namespace, "-l", "app=ueransim")
	cmd := exec.Command(kubectlParts[0], kubectlParts[1:]...)
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get ueransim pod status: %v, output: %s", err, string(output))
	}

	return string(output), nil
}
