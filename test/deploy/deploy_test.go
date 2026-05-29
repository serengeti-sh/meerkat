package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gruntwork-io/terratest/modules/helm"
	http_helper "github.com/gruntwork-io/terratest/modules/http-helper"
	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterName = "meerkat-test"
	namespace   = "meerkat-test"
	imageName   = "meerkat-server"
	imageTag    = "test"
	pgPassword  = "test-deploy-password"
	pgUsername  = "meerkat"
	pgDatabase  = "meerkat"
	healthPath  = "/v1/health"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	// test/deploy/deploy_test.go -> repo root
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return dir
}

func TestDeploy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping deployment test in short mode")
	}

	root := repoRoot(t)
	chartDir := filepath.Join(root, "deployment", "charts", "meerkat")
	valuesFile := filepath.Join(root, "test", "deploy", "test-values.yaml")
	kindConfig := filepath.Join(root, "test", "deploy", "kind-config.yaml")

	// Step 1: Build Docker image
	t.Log("Building Docker image")
	buildImage(t, root)

	// Step 2: Create Kind cluster
	t.Log("Creating Kind cluster")
	createKindCluster(t, kindConfig)
	defer deleteKindCluster(t)

	// Step 3: Load image into Kind
	t.Log("Loading image into Kind")
	loadImage(t)

	// Step 4: Setup kubectl options
	kubectlOptions := k8s.NewKubectlOptions("", "", namespace)

	// Step 5: Create namespace
	t.Logf("Creating namespace: %s", namespace)
	k8s.CreateNamespace(t, kubectlOptions, namespace)
	defer k8s.DeleteNamespace(t, kubectlOptions, namespace)

	// Step 6: Install PostgreSQL
	t.Log("Installing PostgreSQL")
	installPostgres(t, kubectlOptions)
	defer func() {
		helm.Delete(t, &helm.Options{KubectlOptions: kubectlOptions}, "meerkat-postgres", true)
	}()

	// Step 7: Install Meerkat chart
	t.Log("Installing Meerkat Helm chart")
	helmOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
		ValuesFiles:    []string{valuesFile},
	}
	helm.Install(t, helmOptions, chartDir, "meerkat")
	defer helm.Delete(t, helmOptions, "meerkat", true)

	// Step 8: Wait for deployments to be available
	t.Log("Waiting for Meerkat deployments to be available")
	waitForDeployments(t, kubectlOptions)

	// Step 9: Verify health endpoints via port-forward
	t.Log("Verifying health endpoints")
	verifyHealthEndpoints(t, kubectlOptions)

	t.Log("Deployment test PASSED")
}

func buildImage(t *testing.T, repoRoot string) {
	t.Helper()
	cmd := shell.Command{
		Command: "docker",
		Args: []string{
			"build",
			"-f", filepath.Join(repoRoot, "build", "docker", "server.Dockerfile"),
			"-t", fmt.Sprintf("%s:%s", imageName, imageTag),
			repoRoot,
		},
	}
	shell.RunCommand(t, cmd)
}

func createKindCluster(t *testing.T, configPath string) {
	t.Helper()
	// Delete existing cluster if any
	_ = shell.RunCommandE(t, shell.Command{
		Command: "kind",
		Args:    []string{"delete", "cluster", "--name", clusterName},
	})

	cmd := shell.Command{
		Command: "kind",
		Args: []string{
			"create", "cluster",
			"--name", clusterName,
			"--config", configPath,
			"--wait", "120s",
		},
	}
	shell.RunCommand(t, cmd)
}

func deleteKindCluster(t *testing.T) {
	t.Helper()
	t.Log("Deleting Kind cluster")
	cmd := shell.Command{
		Command: "kind",
		Args:    []string{"delete", "cluster", "--name", clusterName},
	}
	// Best-effort cleanup
	_ = shell.RunCommandE(t, cmd)
}

func loadImage(t *testing.T) {
	t.Helper()
	cmd := shell.Command{
		Command: "kind",
		Args: []string{
			"load", "docker-image",
			fmt.Sprintf("%s:%s", imageName, imageTag),
			"--name", clusterName,
		},
	}
	shell.RunCommand(t, cmd)
}

func installPostgres(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()
	// Add Bitnami repo
	helmOptions := &helm.Options{KubectlOptions: kubectlOptions}
	helm.AddRepo(t, helmOptions, "bitnami", "https://charts.bitnami.com/bitnami")

	// Install PostgreSQL with --wait to ensure it's ready
	pgOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
		SetValues: map[string]string{
			"auth.username":                     pgUsername,
			"auth.password":                     pgPassword,
			"auth.database":                     pgDatabase,
			"primary.resources.requests.cpu":    "50m",
			"primary.resources.requests.memory": "128Mi",
			"primary.resources.limits.cpu":      "500m",
			"primary.resources.limits.memory":   "256Mi",
			"volumePermissions.enabled":         "true",
		},
		ExtraArgs: map[string][]string{
			"install": {"--wait", "--timeout", "120s"},
		},
	}
	helm.Install(t, pgOptions, "bitnami/postgresql", "meerkat-postgres")
}

func waitForDeployments(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()
	maxRetries := 60
	retrySleep := 5 * time.Second

	deployments := []string{"meerkat-analyzer", "meerkat-vectors"}

	for i := range maxRetries {
		allReady := true
		for _, name := range deployments {
			deploy, err := k8s.GetDeploymentE(t, kubectlOptions, name)
			if err != nil || !k8s.IsDeploymentAvailable(deploy) {
				allReady = false
				break
			}
		}
		if allReady {
			t.Log("All deployments are available")
			return
		}

		// Log diagnostics every 10 retries (50 seconds)
		if i > 0 && i%10 == 0 {
			t.Logf("=== Diagnostics at retry %d/%d ===", i, maxRetries)
			_ = shell.RunCommandE(t, shell.Command{
				Command: "kubectl",
				Args:    []string{"get", "pods", "-n", namespace, "-o", "wide"},
			})
			_ = shell.RunCommandE(t, shell.Command{
				Command: "kubectl",
				Args:    []string{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
			})
			// Get pod logs (verbose)
			pods := k8s.ListPods(t, kubectlOptions, metav1.ListOptions{})
			for _, pod := range pods {
				if pod.Labels["app.kubernetes.io/name"] == "meerkat" {
					t.Logf("--- Logs for pod %s (phase: %s) ---", pod.Name, pod.Status.Phase)
					_ = shell.RunCommandE(t, shell.Command{
						Command: "kubectl",
						Args:    []string{"logs", "-n", namespace, pod.Name, "--tail=200"},
					})
					// Also get previous container logs if restarted
					for _, cs := range pod.Status.ContainerStatuses {
						if cs.RestartCount > 0 {
							t.Logf("--- Previous container logs for pod %s ---", pod.Name)
							_ = shell.RunCommandE(t, shell.Command{
								Command: "kubectl",
								Args:    []string{"logs", "-n", namespace, pod.Name, "--previous", "--tail=200"},
							})
							break
						}
					}
				}
			}
			// Check rendered configmap
			t.Log("--- ConfigMap data ---")
			_ = shell.RunCommandE(t, shell.Command{
				Command: "kubectl",
				Args:    []string{"get", "configmap", "-n", namespace, "meerkat", "-o", "yaml"},
			})
		}

		t.Logf("Deployments not ready (retry %d/%d), sleeping %v", i+1, maxRetries, retrySleep)
		time.Sleep(retrySleep)
	}

	t.Fatal("Deployments failed to become available within timeout")
}

func verifyHealthEndpoints(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()

	// Verify analyzer health endpoint
	t.Log("Verifying analyzer health endpoint")
	pfCmdAnalyzer := exec.Command("kubectl", "port-forward",
		"-n", namespace,
		"svc/meerkat-analyzer", "18080:8080",
	)
	pfCmdAnalyzer.Stdout = os.Stderr
	pfCmdAnalyzer.Stderr = os.Stderr
	require.NoError(t, pfCmdAnalyzer.Start(), "Failed to start analyzer port-forward")
	defer func() {
		_ = pfCmdAnalyzer.Process.Kill()
		_ = pfCmdAnalyzer.Wait()
	}()

	http_helper.HttpGetWithRetryWithCustomValidation(
		t,
		"http://localhost:18080"+healthPath,
		nil,
		60,
		3*time.Second,
		func(statusCode int, body string) bool {
			return statusCode == 200
		},
	)

	// Verify vectors health endpoint (metrics server)
	t.Log("Verifying vectors health endpoint")
	pfCmdLogs := exec.Command("kubectl", "port-forward",
		"-n", namespace,
		"svc/meerkat-vectors", "19090:9090",
	)
	pfCmdLogs.Stdout = os.Stderr
	pfCmdLogs.Stderr = os.Stderr
	require.NoError(t, pfCmdLogs.Start(), "Failed to start logs port-forward")
	defer func() {
		_ = pfCmdLogs.Process.Kill()
		_ = pfCmdLogs.Wait()
	}()

	http_helper.HttpGetWithRetryWithCustomValidation(
		t,
		"http://localhost:19090/healthz",
		nil,
		60,
		3*time.Second,
		func(statusCode int, body string) bool {
			return statusCode == 200
		},
	)
}
