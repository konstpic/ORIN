package k8s

import (
	"context"
	"errors"
	"io"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/orin/orin/internal/manifest"
)

// ErrPodNotOwned is returned when a pod does not belong to the application (tracking label).
var ErrPodNotOwned = errors.New("pod does not belong to this application")

// GetPodForApp returns the pod if it exists in the namespace and belongs to appName.
//
// Ownership check (in order):
//  1. Pod carries app.kubernetes.io/instance=appName directly (new deployments).
//  2. Fallback: walk ownerReferences (Pod → ReplicaSet → Deployment/StatefulSet/…) to
//     find an ancestor with the tracking label — covers existing pods deployed before the
//     ApplyTracking pod-template fix.
func (cm *ClusterManager) GetPodForApp(ctx context.Context, appName, namespace, podName string) (*corev1.Pod, error) {
	pod, err := cm.clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Fast path: pod carries the tracking label directly.
	if pod.Labels != nil && pod.Labels[manifest.TrackingLabel] == appName {
		return pod, nil
	}

	// Fallback: check whether the app owns a resource in this namespace with the
	// tracking label and that resource is an ancestor of the pod via ownerReferences.
	if cm.podBelongsToApp(ctx, pod, appName, namespace) {
		return pod, nil
	}

	return nil, ErrPodNotOwned
}

// podBelongsToApp returns true when any ownerReference chain from the pod leads
// to a resource carrying app.kubernetes.io/instance=appName.
func (cm *ClusterManager) podBelongsToApp(ctx context.Context, pod *corev1.Pod, appName, namespace string) bool {
	// Collect all resources labelled for this app in the namespace.
	sel := labels.SelectorFromSet(labels.Set{manifest.TrackingLabel: appName})
	ownedUIDs := map[string]struct{}{}
	for _, gvr := range defaultDiscoveryGVRs() {
		objs, err := cm.ListByLabel(gvr, namespace, sel)
		if err != nil {
			continue
		}
		for _, o := range objs {
			ownedUIDs[string(o.GetUID())] = struct{}{}
		}
	}
	// Also include ReplicaSets owned by tracked Deployments (they don't carry
	// the tracking label by default — we need to check their owner chain).
	rsGVR := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	allRS, _ := cm.ListByLabel(rsGVR, namespace, labels.Everything())
	for _, rs := range allRS {
		for _, ref := range rs.GetOwnerReferences() {
			if _, ok := ownedUIDs[string(ref.UID)]; ok {
				ownedUIDs[string(rs.GetUID())] = struct{}{}
				break
			}
		}
	}

	for _, ref := range pod.OwnerReferences {
		if _, ok := ownedUIDs[string(ref.UID)]; ok {
			return true
		}
	}

	// Last resort: look up the pod namespace for the pod with a live API call
	// to handle cases where the informer cache might not have synced ReplicaSets yet.
	for _, ref := range pod.OwnerReferences {
		if ref.Kind != "ReplicaSet" {
			continue
		}
		rs, err := cm.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			continue
		}
		if err == nil {
			if rs.Labels != nil && rs.Labels[manifest.TrackingLabel] == appName {
				return true
			}
			for _, rsOwner := range rs.OwnerReferences {
				if _, ok := ownedUIDs[string(rsOwner.UID)]; ok {
					return true
				}
			}
		}
	}
	return false
}

// ListPodEvents returns recent events for a pod in namespace, newest first.
func (cm *ClusterManager) ListPodEvents(ctx context.Context, namespace, podName string) ([]corev1.Event, error) {
	return cm.ListResourceEvents(ctx, namespace, "Pod", podName)
}

// ListResourceEvents returns recent events for any resource kind in namespace, newest first.
func (cm *ClusterManager) ListResourceEvents(ctx context.Context, namespace, kind, name string) ([]corev1.Event, error) {
	sel := fields.AndSelectors(
		fields.OneTermEqualSelector("involvedObject.kind", kind),
		fields.OneTermEqualSelector("involvedObject.name", name),
	)
	list, err := cm.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{FieldSelector: sel.String()})
	if err != nil {
		return nil, err
	}
	out := append([]corev1.Event(nil), list.Items...)
	sort.Slice(out, func(i, j int) bool {
		li := out[i].LastTimestamp.Time
		lj := out[j].LastTimestamp.Time
		if !li.Equal(lj) {
			return li.After(lj)
		}
		return out[i].Name > out[j].Name
	})
	return out, nil
}

// StreamPodLogs opens a log stream for the pod. Caller must Close the reader.
func (cm *ClusterManager) StreamPodLogs(
	ctx context.Context,
	namespace, podName, container string,
	tailLines int64,
	follow bool,
) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}
	req := cm.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	return req.Stream(ctx)
}

// DetectPodShell tries to find an available shell in the pod container.
// Returns the first working shell from the priority list, or "/bin/sh" as fallback.
func (cm *ClusterManager) DetectPodShell(
	ctx context.Context,
	namespace, podName, container string,
) string {
	shells := []string{
		"/bin/bash",
		"/bin/sh",
		"/bin/ash",
		"sh",
	}

	for _, shell := range shells {
		cmd := []string{"test", "-x", shell}
		var stdout, stderr io.Writer
		err := cm.PodExecStream(ctx, namespace, podName, container, cmd, nil, stdout, stderr, false, 0, 0)
		if err == nil {
			return shell
		}
	}

	// Fallback to /bin/sh if no shell can be detected
	return "/bin/sh"
}

// staticTTYResize implements remotecommand.TerminalSizeQueue with a fixed size (MVP).
type staticTTYResize struct {
	w, h uint16
}

func (s *staticTTYResize) Next() *remotecommand.TerminalSize {
	return &remotecommand.TerminalSize{Width: s.w, Height: s.h}
}

// PodExecStream runs kubectl-style exec until the context is cancelled or the process exits.
func (cm *ClusterManager) PodExecStream(
	ctx context.Context,
	namespace, podName, container string,
	command []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	tty bool,
	termWidth, termHeight int,
) error {
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}
	if termWidth <= 0 {
		termWidth = 120
	}
	if termHeight <= 0 {
		termHeight = 40
	}
	req := cm.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     stdin != nil,
			Stdout:    true,
			Stderr:    true,
			TTY:       tty,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(cm.rest, "POST", req.URL())
	if err != nil {
		return err
	}
	opts := remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	}
	if tty {
		opts.TerminalSizeQueue = &staticTTYResize{w: uint16(termWidth), h: uint16(termHeight)}
	}
	return exec.StreamWithContext(ctx, opts)
}
