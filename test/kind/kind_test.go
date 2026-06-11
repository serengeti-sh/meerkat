package kind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	// Pre-create a static PV for Qdrant to avoid relying on dynamic provisioning
	// in ephemeral CI Kind clusters.
	t.Log("Creating static PV for Qdrant")
	createQdrantPersistentVolume(t)

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

	// Step 7.5: Verify cluster DNS resolution works before installing Meerkat
	t.Log("Verifying cluster DNS resolution")
	verifyDNSResolution(t, kubectlOptions)

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
	// We keep the PVC small and bind it to the static PV we create in this test.
	// We do NOT use --wait because we need to collect diagnostics via the manual
	// wait loop below if the pod fails to become ready.
	cmd = &shell.Command{
		Command: "helm",
		Args: []string{
			"install", "qdrant", "qdrant/qdrant",
			"--namespace", namespace,
			"--set", "persistence.size=100Mi",
			"--set", "persistence.storageClassName=manual",
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

func verifyDNSResolution(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()

	// 1. Check CoreDNS pods
	t.Log("--- Checking CoreDNS pods ---")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"get", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns", "-o", "wide"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})

	// 1a. Check CoreDNS logs and describe for why they're not ready
	t.Log("--- CoreDNS logs ---")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"logs", "-n", "kube-system", "-l", "k8s-app=kube-dns", "--tail=200"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})
	t.Log("--- CoreDNS describe ---")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"describe", "pods", "-n", "kube-system", "-l", "k8s-app=kube-dns"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})
	t.Log("--- kube-dns endpoints ---")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"get", "endpoints", "kube-dns", "-n", "kube-system"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})

	// 2. Check all services in namespace
	t.Log("--- Checking services in namespace ---")
	_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"get", "svc", "-n", namespace},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})

	// 3. Create a temporary busybox pod to verify DNS resolution
	dnsPodYAML := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: dns-verify
  namespace: %s
spec:
  containers:
    - name: dns-verify
      image: busybox:1.36
      command: ["sh", "-c", "while true; do sleep 1; done"]
`, namespace)

	dnsPodFile := filepath.Join(t.TempDir(), "dns-verify.yaml")
	require.NoError(t, os.WriteFile(dnsPodFile, []byte(dnsPodYAML), 0644))

	defer func() {
		_ = shell.RunCommandContextE(t, context.Background(), &shell.Command{
			Command: "kubectl",
			Args:    []string{"delete", "pod", "dns-verify", "-n", namespace, "--ignore-not-found=true", "--force"},
		})
	}()

	shell.RunCommandContext(t, context.Background(), &shell.Command{
		Command:    "kubectl",
		Args:       []string{"apply", "-f", dnsPodFile},
		WorkingDir: ".",
		Env: map[string]string{
			"KUBECONFIG": kubectlOptions.ConfigPath,
		},
	})

	// Wait for the pod to be running
	maxRetries := 60
	for i := range maxRetries {
		pod, err := k8s.GetPodContextE(t, context.Background(), kubectlOptions, "dns-verify")
		if err == nil && pod.Status.Phase == "Running" {
			allReady := true
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					allReady = false
					break
				}
			}
			if allReady {
				break
			}
		}
		t.Logf("DNS verify pod not ready (retry %d/%d), sleeping 5s", i+1, maxRetries)
		time.Sleep(5 * time.Second)
	}

	// 4. DNS resolution check with retries and detailed logging
	services := []string{
		"kubernetes.default",
		"meerkat-postgres-postgresql",
		"qdrant",
	}

	const dnsMaxRetries = 10
	var dnsCompletelyBroken = false
	for _, svc := range services {
		t.Logf("--- DNS check: %s ---", svc)
		resolved := false
		for i := range dnsMaxRetries {
			out, err := shell.RunCommandContextAndGetOutputE(t, context.Background(), &shell.Command{
				Command: "kubectl",
				Args:    []string{"exec", "dns-verify", "-n", namespace, "--", "sh", "-c", fmt.Sprintf("nslookup %s 2>&1 || true", svc)},
				Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
			})
			// nslookup returns exit 1 even when successful due to search suffix
			// We check the output regardless of kubectl exec error (which should be nil due to || true)
			if err != nil {
				if i == 0 || i == dnsMaxRetries-1 {
					t.Logf("  %s kubectl exec error: %v", svc, err)
				}
			} else if strings.Contains(out, "Address:") {
				// Extract the IP address from output
				lines := strings.Split(out, "\n")
				for _, line := range lines {
					if strings.Contains(line, "Address:") && !strings.Contains(line, "10.96.0.10") {
						t.Logf("  %s -> %s", svc, strings.TrimSpace(line))
						resolved = true
						break
					}
				}
				if resolved {
					break
				}
			} else {
				if i == 0 || i == dnsMaxRetries-1 {
					t.Logf("  %s DNS attempt %d/%d: output=%s", svc, i+1, dnsMaxRetries, out)
				}
			}
			if strings.Contains(out, "connection timed out") || strings.Contains(out, "no servers could be reached") {
				dnsCompletelyBroken = true
			}
			if i < dnsMaxRetries-1 {
				time.Sleep(2 * time.Second)
			}
		}
		if !resolved {
			t.Logf("  WARNING: DNS resolution for %s not confirmed after %d retries", svc, dnsMaxRetries)
		}
		// If kubernetes.default can't resolve, DNS is fundamentally broken; skip remaining checks
		if svc == "kubernetes.default" && !resolved {
			t.Log("  FUNDAMENTAL DNS FAILURE: kubernetes.default cannot resolve. Skipping remaining DNS checks.")
			break
		}
	}

	if dnsCompletelyBroken {
		t.Log("\n=== DNS IS COMPLETELY BROKEN ===")
		t.Log("CoreDNS pods are not responding. This is likely due to:")
		t.Log("  1. CoreDNS readiness probe failure (pods shown as 0/1)")
		t.Log("  2. OOMKill or resource starvation on the Kind node")
		t.Log("  3. Network plugin (CNI) not fully initialized")
		t.Log("CoreDNS logs and describe output were captured above.")
		t.Log("Aborting test early because DNS-dependent services cannot start.")
		t.Fatal("Cluster DNS is not functional. See CoreDNS logs above.")
	}

	// 5. Check /etc/resolv.conf for debugging
	t.Log("--- DNS verify pod /etc/resolv.conf ---")
	out, _ := shell.RunCommandContextAndGetOutputE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"exec", "dns-verify", "-n", namespace, "--", "cat", "/etc/resolv.conf"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})
	t.Logf("  resolv.conf: %s", out)

	// 6. Check if CoreDNS is actually responding (avoid busybox nc -zv which hangs)
	t.Log("--- CoreDNS connectivity test ---")
	out, _ := shell.RunCommandContextAndGetOutputE(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"exec", "dns-verify", "-n", namespace, "--", "sh", "-c", "echo | nc -w 3 10.96.0.10 53 2>&1 || true"},
		Env:     map[string]string{"KUBECONFIG": kubectlOptions.ConfigPath},
	})
	t.Logf("  CoreDNS connect: %s", out)
}

func createQdrantPersistentVolume(t *testing.T) {
	t.Helper()

	manifest := `apiVersion: v1
kind: PersistentVolume
metadata:
  name: qdrant-storage-pv
spec:
  capacity:
    storage: 100Mi
  accessModes:
    - ReadWriteOnce
  storageClassName: manual
  persistentVolumeReclaimPolicy: Delete
  hostPath:
    path: /tmp/qdrant-storage
    type: DirectoryOrCreate
`

	pvFile := filepath.Join(t.TempDir(), "qdrant-pv.yaml")
	require.NoError(t, os.WriteFile(pvFile, []byte(manifest), 0o600))

	shell.RunCommandContext(t, context.Background(), &shell.Command{
		Command: "kubectl",
		Args:    []string{"apply", "-f", pvFile},
	})
}

func waitForDeployments(t *testing.T, kubectlOptions *k8s.KubectlOptions) {
	t.Helper()
	maxRetries := 90
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
