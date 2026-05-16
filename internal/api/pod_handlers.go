package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/k8s-ui/k8s-ui/internal/k8s"
	"github.com/k8s-ui/k8s-ui/internal/rbac"
	"github.com/k8s-ui/k8s-ui/internal/rbacenforce"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

// categorizeEvent maps a Kubernetes event reason to a UI-friendly category
func categorizeEvent(reason, resourceKind string) string {
	switch reason {
	// Pod lifecycle events
	case "Created", "Scheduled", "Triggered":
		return "PodCreated"
	case "Pulling", "PullingImage":
		return "PodStarting"
	case "Pulled":
		return "ImagePulled"
	case "Killing", "Finished":
		return "PodStopping"

	// Container lifecycle events
	case "ContainerStarted", "ContainerCreated":
		return "ContainerStarted"
	case "ContainerFinished", "ContainerStopped":
		return "ContainerStopped"
	case "ContainerCrashed", "BackOff":
		return "ContainerCrash"

	// Image pull events
	case "ImagePullBackOff", "ErrImagePull", "FailedToStartContainer":
		return "ImagePullFailed"
	case "SuccessfullyPulledImage", "PulledImage":
		return "ImagePullSuccess"

	// Probe events
	case "LivenessProbe", "Unhealthy":
		return "LivenessProbe"
	case "ReadinessProbe":
		return "ReadinessProbe"
	case "StartupProbe":
		return "StartupProbe"

	// Resource events
	case "FailedScheduling", "FailedAdmission":
		return "SchedulingFailed"
	case "SuccessfulCreate", "SuccessfulDelete":
		return "SuccessfulOperation"
	case "Failed", "FailedCreate", "FailedDelete":
		return "OperationFailed"

	// Mount events
	case "FailedMount", "FailedAttachVolume":
		return "MountFailed"
	case "SuccessfulMountVolume":
		return "MountSuccess"

	// Network events
	case "FailedToCreateNetwork":
		return "NetworkFailed"

	default:
		return "Other"
	}
}

// getApplicationPod returns pod metadata for the UI (containers, phase).
func (s *Server) getApplicationPod(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}
	out := apiv1.PodSummary{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Phase:     string(pod.Status.Phase),
	}
	for _, c := range pod.Spec.Containers {
		out.Containers = append(out.Containers, apiv1.PodContainer{Name: c.Name})
	}
	for _, c := range pod.Spec.InitContainers {
		out.InitContainers = append(out.InitContainers, apiv1.PodContainer{Name: c.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// getApplicationPodEvents returns recent Kubernetes events for the pod.
func (s *Server) getApplicationPodEvents(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}
	evs, err := s.opts.Cluster.ListPodEvents(r.Context(), pod.Namespace, pod.Name)
	if err != nil {
		writeError(w, http.StatusBadGateway, "events_list_failed", err.Error())
		return
	}
	out := make([]apiv1.PodEvent, 0, len(evs))
	for _, e := range evs {
		pe := apiv1.PodEvent{
			Type:         e.Type,
			Reason:       e.Reason,
			Message:      e.Message,
			Count:        e.Count,
			Category:     categorizeEvent(e.Reason, e.InvolvedObject.Kind),
			ResourceKind: e.InvolvedObject.Kind,
			ResourceName: e.InvolvedObject.Name,
			Namespace:    e.InvolvedObject.Namespace,
		}
		if !e.FirstTimestamp.IsZero() {
			t := e.FirstTimestamp.Time
			pe.FirstTime = &t
		}
		if !e.LastTimestamp.IsZero() {
			t := e.LastTimestamp.Time
			pe.LastTime = &t
		}
		out = append(out, pe)
	}
	writeJSON(w, http.StatusOK, out)
}

// getApplicationResourceEvents returns recent Kubernetes events for any resource
// (Deployment, ReplicaSet, etc.) belonging to the application.
// Query params: kind, name, namespace.
func (s *Server) getApplicationResourceEvents(w http.ResponseWriter, r *http.Request) {
	appName := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, appName)
	if !ok {
		return
	}

	kind := r.URL.Query().Get("kind")
	resName := r.URL.Query().Get("name")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = app.DestNamespace
	}
	if kind == "" || resName == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "kind and name query params are required")
		return
	}

	evs, err := s.opts.Cluster.ListResourceEvents(r.Context(), namespace, kind, resName)
	if err != nil {
		writeError(w, http.StatusBadGateway, "events_list_failed", err.Error())
		return
	}
	out := make([]apiv1.PodEvent, 0, len(evs))
	for _, e := range evs {
		pe := apiv1.PodEvent{
			Type:         e.Type,
			Reason:       e.Reason,
			Message:      e.Message,
			Count:        e.Count,
			Category:     categorizeEvent(e.Reason, e.InvolvedObject.Kind),
			ResourceKind: e.InvolvedObject.Kind,
			ResourceName: e.InvolvedObject.Name,
			Namespace:    e.InvolvedObject.Namespace,
		}
		if !e.FirstTimestamp.IsZero() {
			t := e.FirstTimestamp.Time
			pe.FirstTime = &t
		}
		if !e.LastTimestamp.IsZero() {
			t := e.LastTimestamp.Time
			pe.LastTime = &t
		}
		out = append(out, pe)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getApplicationPodLog(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}

	container := r.URL.Query().Get("container")
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}
	if container == "" {
		writeError(w, http.StatusBadRequest, "no_container", "pod has no containers")
		return
	}

	tailLines := int64(500)
	if v := r.URL.Query().Get("tailLines"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			tailLines = n
		}
	}
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"

	stream, err := s.opts.Cluster.StreamPodLogs(r.Context(), app.DestNamespace, podName, container, tailLines, follow)
	if err != nil {
		writeError(w, http.StatusBadGateway, "log_stream_failed", err.Error())
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if follow {
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
		_, _ = io.Copy(&flushWriter{w: w, f: flusher}, stream)
		return
	}
	_, _ = io.Copy(w, stream)
}

type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return n, err
}

// getApplicationPodShell detects and returns the available shell in the pod.
func (s *Server) getApplicationPodShell(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermPodShell) {
		return
	}
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}

	container := r.URL.Query().Get("container")
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}
	if container == "" {
		writeError(w, http.StatusBadRequest, "no_container", "pod has no containers")
		return
	}

	// Detect available shell with a timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	shell := s.opts.Cluster.DetectPodShell(ctx, pod.Namespace, pod.Name, container)

	writeJSON(w, http.StatusOK, map[string]string{"shell": shell})
}

// appPodExecWS streams pod exec over WebSocket.
// Client → server: TextMessage JSON {"resize":{"w":120,"h":40}} or BinaryMessage = raw stdin bytes.
// Server → client: BinaryMessage [0x01|0x02][payload] for stdout|stderr.
func (s *Server) appPodExecWS(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermPodExec) {
		return
	}
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}

	container := r.URL.Query().Get("container")
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}
	if container == "" {
		writeError(w, http.StatusBadRequest, "no_container", "pod has no containers")
		return
	}

	cmd := r.URL.Query().Get("command")
	if cmd == "" {
		cmd = "/bin/sh"
	}
	command := strings.Fields(cmd)
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}

	tw, _ := strconv.Atoi(r.URL.Query().Get("w"))
	th, _ := strconv.Atoi(r.URL.Query().Get("h"))
	if tw <= 0 {
		tw = 120
	}
	if th <= 0 {
		th = 40
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	stdinR, stdinW := io.Pipe()
	var mu sync.Mutex
	writeBin := func(prefix byte, p []byte) {
		if len(p) == 0 {
			return
		}
		out := make([]byte, 1+len(p))
		out[0] = prefix
		copy(out[1:], p)
		mu.Lock()
		defer mu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		_ = conn.WriteMessage(websocket.BinaryMessage, out)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		defer stdinW.Close()
		for {
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
			mt, data, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			if mt == websocket.TextMessage {
				// optional resize — ignored in MVP static TTY; reserved for future
				continue
			}
			if mt == websocket.BinaryMessage && len(data) > 0 {
				_, _ = stdinW.Write(data)
			}
		}
	}()

	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	go func() {
		buf := make([]byte, 16*1024)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				writeBin(1, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 8*1024)
		for {
			n, err := stderrR.Read(buf)
			if n > 0 {
				writeBin(2, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	err = s.opts.Cluster.PodExecStream(ctx, app.DestNamespace, podName, container, command, stdinR, stdoutW, stderrW, true, tw, th)
	_ = stdoutW.Close()
	_ = stderrW.Close()
	if err != nil && ctx.Err() == nil {
		mu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+escapeJSON(err.Error())+`"}`))
		mu.Unlock()
	}
}

// deleteApplicationPod deletes (kills) the pod. Kubernetes will restart it via
// the owning controller (Deployment, StatefulSet, etc.).
func (s *Server) deleteApplicationPod(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermPodDelete) {
		return
	}
	name := chi.URLParam(r, "name")
	podName := chi.URLParam(r, "pod")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	pod, err := s.opts.Cluster.GetPodForApp(r.Context(), app.Name, app.DestNamespace, podName)
	if errors.Is(err, k8s.ErrPodNotOwned) {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "not_found", "pod not found")
			return
		}
		writeError(w, http.StatusBadGateway, "pod_get_failed", err.Error())
		return
	}
	gvk := pod.GroupVersionKind()
	if gvk.Version == "" {
		gvk.Version = "v1"
	}
	if gvk.Kind == "" {
		gvk.Kind = "Pod"
	}
	if err := s.opts.Cluster.Delete(r.Context(), gvk, pod.Namespace, pod.Name); err != nil {
		writeError(w, http.StatusBadGateway, "delete_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(name)
	w.WriteHeader(http.StatusNoContent)
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
