package kubernetes

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkspaceOptions defines options for creating a workspace
type WorkspaceOptions struct {
	Name       string
	GitURL     string
	Branch     string
	Image      string
	CPU        string
	Memory     string
	Storage    string
	EnableDinD bool
	SSHPubKey  string // Optional: SSH public key to inject
}

// Workspace represents a workspace status
type Workspace struct {
	Name   string
	Status string
	Age    string
	GitURL string
	PodIP  string
}

// DeleteOptions defines options for deleting a workspace
type DeleteOptions struct {
	Name    string
	KeepPVC bool
}

// CreateWorkspace creates a new workspace in Kubernetes
func (c *Client) CreateWorkspace(ctx context.Context, opts WorkspaceOptions) (*Workspace, error) {
	// Ensure namespace exists
	if err := c.EnsureNamespace(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace: %w", err)
	}

	podName := "ws-" + opts.Name
	pvcName := podName + "-pvc"
	secretName := podName + "-ssh"

	// Check if workspace already exists
	_, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("workspace '%s' already exists", opts.Name)
	}

	// Create PVC for workspace storage
	pvc := buildPVC(pvcName, opts)
	_, err = c.clientset.CoreV1().PersistentVolumeClaims(WorkspaceNamespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create PVC: %w", err)
	}

	// Create SSH keys secret (placeholder for now)
	secret := buildSSHSecret(secretName, opts)
	_, err = c.clientset.CoreV1().Secrets(WorkspaceNamespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("failed to create secret: %w", err)
	}

	// Create the pod
	pod := buildPod(podName, pvcName, secretName, opts)
	createdPod, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %w", err)
	}

	return &Workspace{
		Name:   opts.Name,
		Status: string(createdPod.Status.Phase),
		GitURL: opts.GitURL,
	}, nil
}

// GetWorkspace retrieves a workspace by name
func (c *Client) GetWorkspace(ctx context.Context, name string) (*Workspace, error) {
	podName := "ws-" + name

	pod, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("workspace '%s' not found", name)
		}
		return nil, err
	}

	return podToWorkspace(pod), nil
}

// ListWorkspaces lists all workspaces
func (c *Client) ListWorkspaces(ctx context.Context, includeAll bool) ([]Workspace, error) {
	labelSelector := fmt.Sprintf("%s", WorkspaceLabel)

	pods, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		if errors.IsNotFound(err) {
			return []Workspace{}, nil
		}
		return nil, err
	}

	workspaces := make([]Workspace, 0, len(pods.Items))
	for _, pod := range pods.Items {
		ws := podToWorkspace(&pod)
		// Filter out completed/failed pods unless includeAll
		if !includeAll && (ws.Status == "Succeeded" || ws.Status == "Failed") {
			continue
		}
		workspaces = append(workspaces, *ws)
	}

	return workspaces, nil
}

// DeleteWorkspace deletes a workspace
func (c *Client) DeleteWorkspace(ctx context.Context, opts DeleteOptions) error {
	podName := "ws-" + opts.Name
	pvcName := podName + "-pvc"
	secretName := podName + "-ssh"

	// Delete pod
	err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	// Delete secret
	err = c.clientset.CoreV1().Secrets(WorkspaceNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Delete PVC unless keepPVC is set
	if !opts.KeepPVC {
		err = c.clientset.CoreV1().PersistentVolumeClaims(WorkspaceNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete PVC: %w", err)
		}
	}

	return nil
}

// StopWorkspace stops a workspace by deleting the pod (keeping PVC)
func (c *Client) StopWorkspace(ctx context.Context, name string) error {
	podName := "ws-" + name

	err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("workspace '%s' not found or already stopped", name)
		}
		return fmt.Errorf("failed to stop workspace: %w", err)
	}

	return nil
}

// StartWorkspace starts a stopped workspace
func (c *Client) StartWorkspace(ctx context.Context, name string) error {
	podName := "ws-" + name
	pvcName := podName + "-pvc"
	secretName := podName + "-ssh"

	// Check if pod already exists
	_, err := c.clientset.CoreV1().Pods(WorkspaceNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("workspace '%s' is already running", name)
	}

	// Check if PVC exists (to get workspace config)
	pvc, err := c.clientset.CoreV1().PersistentVolumeClaims(WorkspaceNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("workspace '%s' not found (no PVC)", name)
		}
		return err
	}

	// Reconstruct options from PVC labels/annotations
	opts := WorkspaceOptions{
		Name:       name,
		GitURL:     pvc.Annotations[GitURLAnnotation],
		Branch:     pvc.Annotations[BranchAnnotation],
		Image:      "justup/devcontainer:latest",
		CPU:        "1",
		Memory:     "2Gi",
		EnableDinD: pvc.Labels["justup.io/dind"] == "true",
	}

	// Recreate the pod
	pod := buildPod(podName, pvcName, secretName, opts)
	_, err = c.clientset.CoreV1().Pods(WorkspaceNamespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	return nil
}

// podToWorkspace converts a pod to a Workspace struct
func podToWorkspace(pod *corev1.Pod) *Workspace {
	name := pod.Labels[WorkspaceLabel]
	if name == "" {
		// Fallback: extract from pod name
		if len(pod.Name) > 3 {
			name = pod.Name[3:] // Remove "ws-" prefix
		}
	}

	age := formatAge(pod.CreationTimestamp.Time)
	gitURL := pod.Annotations[GitURLAnnotation]

	return &Workspace{
		Name:   name,
		Status: string(pod.Status.Phase),
		Age:    age,
		GitURL: gitURL,
		PodIP:  pod.Status.PodIP,
	}
}

// formatAge formats a time as a human-readable age
func formatAge(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// buildPVC creates a PersistentVolumeClaim spec
func buildPVC(name string, opts WorkspaceOptions) *corev1.PersistentVolumeClaim {
	storageQty := resource.MustParse(opts.Storage)

	labels := map[string]string{
		WorkspaceLabel: opts.Name,
	}
	if opts.EnableDinD {
		labels["justup.io/dind"] = "true"
	}

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: WorkspaceNamespace,
			Labels:    labels,
			Annotations: map[string]string{
				GitURLAnnotation: opts.GitURL,
				BranchAnnotation: opts.Branch,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQty,
				},
			},
		},
	}
}

// buildSSHSecret creates a Secret for SSH keys
func buildSSHSecret(name string, opts WorkspaceOptions) *corev1.Secret {
	authorizedKeys := opts.SSHPubKey
	if authorizedKeys == "" {
		// TODO: Get from justup config/database
		authorizedKeys = ""
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: WorkspaceNamespace,
			Labels: map[string]string{
				WorkspaceLabel: opts.Name,
			},
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"authorized_keys": authorizedKeys,
		},
	}
}

// buildPod creates a Pod spec for the workspace
func buildPod(podName, pvcName, secretName string, opts WorkspaceOptions) *corev1.Pod {
	cpuQty := resource.MustParse(opts.CPU)
	memQty := resource.MustParse(opts.Memory)

	// Main workspace container
	workspaceContainer := corev1.Container{
		Name:            "workspace",
		Image:           opts.Image,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{
				Name:          "ssh",
				ContainerPort: 22,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuQty,
				corev1.ResourceMemory: memQty,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuQty,
				corev1.ResourceMemory: memQty,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "workspace",
				MountPath: "/home/dev/workspace",
			},
			{
				Name:      "ssh-keys",
				MountPath: "/etc/justup/ssh-keys",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{Name: "JUSTUP_WORKSPACE", Value: opts.Name},
			{Name: "GIT_URL", Value: opts.GitURL},
			{Name: "GIT_BRANCH", Value: opts.Branch},
		},
	}

	// Add Docker connection if DinD is enabled
	// Use TCP connection to the DinD sidecar (more reliable than socket sharing)
	if opts.EnableDinD {
		workspaceContainer.Env = append(workspaceContainer.Env, corev1.EnvVar{
			Name:  "DOCKER_HOST",
			Value: "tcp://localhost:2375",
		})
	}

	containers := []corev1.Container{workspaceContainer}

	// Add DinD sidecar if enabled
	if opts.EnableDinD {
		dindContainer := corev1.Container{
			Name:  "dind",
			Image: "docker:24-dind",
			SecurityContext: &corev1.SecurityContext{
				Privileged: boolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "docker-socket",
					MountPath: "/var/run",
				},
				{
					Name:      "docker-storage",
					MountPath: "/var/lib/docker",
				},
			},
			Env: []corev1.EnvVar{
				{Name: "DOCKER_TLS_CERTDIR", Value: ""},
			},
		}
		containers = append(containers, dindContainer)
	}

	// Init container to clone the repository
	initContainers := []corev1.Container{
		{
			Name:  "git-clone",
			Image: "alpine/git:latest",
			Command: []string{"/bin/sh", "-c"},
			Args: []string{
				fmt.Sprintf(`
					if [ ! -d /workspace/.git ]; then
						echo "Cloning repository..."
						git clone --branch %s %s /workspace
					else
						echo "Repository already exists, skipping clone"
					fi
				`, opts.Branch, opts.GitURL),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "workspace",
					MountPath: "/workspace",
				},
			},
			Env: []corev1.EnvVar{
				{Name: "GIT_SSH_COMMAND", Value: "ssh -o StrictHostKeyChecking=no"},
			},
		},
	}

	volumes := []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		{
			Name: "ssh-keys",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: int32Ptr(0600),
				},
			},
		},
	}

	if opts.EnableDinD {
		volumes = append(volumes,
			corev1.Volume{
				Name: "docker-socket",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: "docker-storage",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: WorkspaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":     "justup-workspace",
				"app.kubernetes.io/instance": opts.Name,
				WorkspaceLabel:               opts.Name,
			},
			Annotations: map[string]string{
				GitURLAnnotation: opts.GitURL,
				BranchAnnotation: opts.Branch,
				// Disable AppArmor for SSH to work properly in containers
				"container.apparmor.security.beta.kubernetes.io/workspace": "unconfined",
			},
		},
		Spec: corev1.PodSpec{
			InitContainers:                initContainers,
			Containers:                    containers,
			Volumes:                       volumes,
			RestartPolicy:                 corev1.RestartPolicyAlways,
			TerminationGracePeriodSeconds: int64Ptr(30),
		},
	}
}

func boolPtr(b bool) *bool       { return &b }
func int32Ptr(i int32) *int32    { return &i }
func int64Ptr(i int64) *int64    { return &i }
