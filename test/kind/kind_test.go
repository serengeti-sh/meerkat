package kind

import (
	"context"
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
	// test/kind/kind_test.go -> repo root
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
	valuesFile := filepath.Join(root, "test", "kind", "test-values.yaml")
	kindConfig := filepath.Join(root, "test", "kind", "kind-config.yaml")

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
	k8s.CreateNamespaceContext(t, context.Background(), kubectlOptions, namespace)
	defer k8s.DeleteNamespaceContext(t, context.Background(), kubectlOptions, namespace)

	// Step 6: Install PostgreSQL
	t.Log("Installing PostgreSQL")
	installPostgres(t, kubectlOptions)
	defer func() {
		_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
			Command: "kubectl",
			Args:    []string{"delete", "-n", namespace, "statefulset/postgres", "svc/meerkat-postgres-postgresql", "--ignore-not-found=true"},
		})
	}()

	// Step 7: Install Qdrant for vector store
	t.Log("Installing Qdrant")
	installQdrant(t, kubectlOptions)
	defer func() {
		_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
			Command: "helm",
			Args:    []string{"delete", "--namespace", namespace, "qdrant"},
		})
	}()

	// Step 8: Install Meerkat chart
	t.Log("Installing Meerkat Helm chart")
	helmOptions := &helm.Options{
		KubectlOptions: kubectlOptions,
		ValuesFiles:    []string{valuesFile},
	}
	helm.InstallContext(t, context.Background(), helmOptions, chartDir, "meerkat")
	defer helm.DeleteContext(t, context.Background(), helmOptions, "meerkat", true)

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
	cmd := &shell.Command{
		Command: "docker",
		Args: []string{
			"build",
			"-f", filepath.Join(repoRoot, "build", "docker", "server.Dockerfile"),
			"-t", fmt.Sprintf("%s:%s", imageName, imageTag),
			repoRoot,
		},
	}
	shell.RunCommandContext(t, context.Background(), cmd)
}

func createKindCluster(t *testing.T, configPath string) {
	t.Helper()
	// Delete existing cluster if any
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kind",
		Args:    []string{"delete", "cluster", "--name", clusterName},
	})
	// Give Docker a moment to release resources before recreating.
	time.Sleep(5 * time.Second)

	cmd := &shell.Command{
		Command: "kind",
		Args: []string{
			"create", "cluster",
			"--name", clusterName,
			"--config", configPath,
			"--wait", "300s",
		},
	}

	const maxRetries = 3
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			t.Logf("Kind cluster creation failed, retrying (%d/%d) in 10s...", i, maxRetries-1)
			time.Sleep(10 * time.Second)
		}
		lastErr = shell.RunCommandContextE(t, context.Background(), cmd)
		if lastErr == nil {
			return
		}
	}
	t.Fatalf("Failed to create Kind cluster after %d attempts: %v", maxRetries, lastErr)
}

func deleteKindCluster(t *testing.T) {
	t.Helper()
	t.Log("Deleting Kind cluster")
	cmd := &shell.Command{
		Command: "kind",
		Args:    []string{"delete", "cluster", "--name", clusterName},
	}
	// Best-effort cleanup
	_ = shell.RunCommandContextE(t, context.Background(), cmd)
}

func loadImage(t *testing.T) {
	t.Helper()
	cmd := &shell.Command{
		Command: "kind",
		Args: []string{
			"load", "docker-image",
			fmt.Sprintf("%s:%s", imageName, imageTag),
			"--name", clusterName,
		},
	}
	shell.RunCommandContext(t, context.Background(), cmd)
}

func installPostgres(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()
	pgYAML := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: meerkat-postgres-postgresql
  namespace: %s
spec:
  ports:
    - port: 5432
      targetPort: 5432
  selector:
    app: postgres
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: %s
spec:
  serviceName: meerkat-postgres-postgresql
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
        - name: postgres
          image: postgres:17-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              value: %s
            - name: POSTGRES_PASSWORD
              value: %s
            - name: POSTGRES_DB
              value: %s
            - name: PGDATA
              value: /var/lib/postgresql/data/pgdata
          resources:
            requests:
              cpu: 50m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          readinessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - %s
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 3
            failureThreshold: 30
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
      volumes:
        - name: data
          emptyDir: {}
`, namespace, namespace, pgUsername, pgPassword, pgDatabase, pgUsername)

	// Write YAML to temp file and apply
	tmpFile := filepath.Join(t.TempDir(), "postgres.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(pgYAML), 0644))

	shell.RunCommandContext(t, context.Background(), &shell.Command{
		Command:    "kubectl",
		Args:       []string{"apply", "-f", tmpFile},
		WorkingDir: ".",
		Env: map[string]string{
			"KUBECONFIG": kubectlOptions.ConfigPath,
		},
	})

	// Wait for PostgreSQL pod to be ready
	maxRetries := 60
	for i := range maxRetries {
		pods := k8s.ListPodsContext(t, context.Background(), kubectlOptions, metav1.ListOptions{LabelSelector: "app=postgres"})
		if len(pods) > 0 && pods[0].Status.Phase == "Running" {
			allReady := true
			for _, cs := range pods[0].Status.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				t.Log("PostgreSQL is ready")
				return
			}
		}
		t.Logf("PostgreSQL not ready (retry %d/%d), sleeping 5s", i+1, maxRetries)
		time.Sleep(5 * time.Second)
	}

	// Print diagnostics on failure
	t.Log("=== PostgreSQL failure diagnostics ===")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"describe", "pod", "-l", "app=postgres", "-n", namespace},
	})
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"logs", "-l", "app=postgres", "-n", namespace, "--tail=200"},
	})
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
	})

	t.Fatal("PostgreSQL failed to become ready within timeout")
}

func installQdrant(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()

	// Add Qdrant Helm repo
	cmd := &shell.Command{
		Command: "helm",
		Args:    []string{"repo", "add", "qdrant", "https://qdrant.github.io/qdrant-helm"},
	}
	_ = shell.RunCommandContextE(t, context.Background(), cmd)

	cmd = &shell.Command{
		Command: "helm",
		Args:    []string{"repo", "update"},
	}
	shell.RunCommandContext(t, context.Background(), cmd)

	// Install Qdrant
	// Note: Qdrant chart always creates a PVC; there is no persistence.enabled.
	// We shrink the PVC to 100Mi so the Kind local-path provisioner binds it
	// quickly. We do NOT use --wait because we need to collect diagnostics via
	// the manual wait loop below if the pod fails to become ready.
	cmd = &shell.Command{
		Command: "helm",
		Args: []string{
			"install", "qdrant", "qdrant/qdrant",
			"--namespace", namespace,
			"--set", "persistence.size=100Mi",
			"--set", "resources.requests.cpu=50m",
			"--set", "resources.requests.memory=128Mi",
			"--set", "resources.limits.cpu=500m",
			"--set", "resources.limits.memory=256Mi",
		},
	}
	shell.RunCommandContext(t, context.Background(), cmd)

	// Wait for Qdrant to be ready (10 min max = 120 retries × 5s)
	maxRetries := 120
	for i := range maxRetries {
		pods := k8s.ListPodsContext(t, context.Background(), kubectlOptions, metav1.ListOptions{LabelSelector: "app.kubernetes.io/name=qdrant"})
		if len(pods) > 0 && pods[0].Status.Phase == "Running" {
			allReady := true
			for _, cs := range pods[0].Status.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				t.Log("Qdrant is ready")
				return
			}
		}
		t.Logf("Qdrant not ready (retry %d/%d), sleeping 5s", i+1, maxRetries)
		time.Sleep(5 * time.Second)
	}

	// Print diagnostics on failure
	t.Log("=== Qdrant failure diagnostics ===")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"describe", "pod", "-l", "app.kubernetes.io/name=qdrant", "-n", namespace},
	})
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"logs", "-l", "app.kubernetes.io/name=qdrant", "-n", namespace, "--tail=200"},
	})
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
	})

	t.Fatal("Qdrant failed to become ready within timeout")
}

func waitForDeployments(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()
	maxRetries := 60
	retrySleep := 5 * time.Second

	deployments := []string{"meerkat-analyzer", "meerkat-vectors"}

	for i := range maxRetries {
		allReady := true
		for _, name := range deployments {
			deploy, err := k8s.GetDeploymentContextE(t, context.Background(), kubectlOptions, name)
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
			_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
				Command: "kubectl",
				Args:    []string{"get", "pods", "-n", namespace, "-o", "wide"},
			})
			_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
				Command: "kubectl",
				Args:    []string{"get", "events", "-n", namespace, "--sort-by=.lastTimestamp"},
			})
			// Get pod logs (verbose)
			pods := k8s.ListPodsContext(t, context.Background(), kubectlOptions, metav1.ListOptions{})
			for _, pod := range pods {
				if pod.Labels["app.kubernetes.io/name"] == "meerkat" {
					t.Logf("--- Logs for pod %s (phase: %s) ---", pod.Name, pod.Status.Phase)
					_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
						Command: "kubectl",
						Args:    []string{"logs", "-n", namespace, pod.Name, "--tail=200"},
					})
					// Also get previous container logs if restarted
					for _, cs := range pod.Status.ContainerStatuses {
						if cs.RestartCount > 0 {
							t.Logf("--- Previous container logs for pod %s ---", pod.Name)
							_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
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
			_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
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

	http_helper.HTTPGetWithRetryWithCustomValidationContext(
		t,
		context.Background(),
		"http://localhost:18080"+healthPath,
		nil,
		60,
		3*time.Second,
		func(statusCode int, body string) bool {
			return statusCode == 200
		},
	)

	// Verify vectors health endpoint
	t.Log("Verifying vectors health endpoint")
	pfCmdVectors := exec.Command("kubectl", "port-forward",
		"-n", namespace,
		"svc/meerkat-vectors", "19090:9090",
	)
	pfCmdVectors.Stdout = os.Stderr
	pfCmdVectors.Stderr = os.Stderr
	require.NoError(t, pfCmdVectors.Start(), "Failed to start vectors port-forward")
	defer func() {
		_ = pfCmdVectors.Process.Kill()
		_ = pfCmdVectors.Wait()
	}()

	http_helper.HTTPGetWithRetryWithCustomValidationContext(
		t,
		context.Background(),
		"http://localhost:19090/healthz",
		nil,
		60,
		3*time.Second,
		func(statusCode int, body string) bool {
			return statusCode == 200
		},
	)
}
