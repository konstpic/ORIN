package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/k8s"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) listClusterHealth(w http.ResponseWriter, r *http.Request) {
	clusters, err := s.opts.Store.Clusters.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	apps, _ := s.opts.Store.Applications.List(r.Context())

	out := make([]apiv1.ClusterHealth, 0, len(clusters))
	for _, cl := range clusters {
		h := probeCluster(r.Context(), cl, apps, s.opts.Cluster)
		out = append(out, h)
	}
	writeJSON(w, http.StatusOK, out)
}

func probeCluster(ctx context.Context, cl *domain.Cluster, apps []*domain.Application, cm *k8s.ClusterManager) apiv1.ClusterHealth {
	h := apiv1.ClusterHealth{
		ClusterID:   cl.ID,
		ClusterName: cl.Name,
		Status:      "Unreachable",
	}

	// Count apps targeting this cluster
	for _, app := range apps {
		if app.DestClusterID == cl.ID {
			h.AppCount++
		}
	}

	if cl.InCluster {
		if cm == nil {
			h.Error = "no cluster manager"
			return h
		}
		cs := cm.Clientset()
		ver, err := cs.Discovery().ServerVersion()
		if err != nil {
			h.Error = err.Error()
			return h
		}
		h.K8sVersion = ver.GitVersion
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			h.Error = err.Error()
			return h
		}
		h.NodeCount = len(nodes.Items)
		readyNodes := 0
		for _, n := range nodes.Items {
			for _, cond := range n.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
					readyNodes++
					break
				}
			}
		}
		if readyNodes == len(nodes.Items) {
			h.Status = "Ready"
		} else if readyNodes > 0 {
			h.Status = "Degraded"
		}
		return h
	}

	// Remote cluster
	rc, err := k8s.NewRemoteClusterFromKubeconfigYAML(string(cl.AuthConfigEncrypted))
	if err != nil {
		h.Error = err.Error()
		return h
	}
	cs := rc.Clientset()
	ver, err := cs.Discovery().ServerVersion()
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.K8sVersion = ver.GitVersion
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		h.Error = err.Error()
		return h
	}
	h.NodeCount = len(nodes.Items)
	h.Status = "Ready"
	return h
}

func (s *Server) listClusterNodes(w http.ResponseWriter, r *http.Request) {
	clID := chi.URLParam(r, "id")
	cl, err := s.opts.Store.Clusters.GetByID(r.Context(), clID)
	if err != nil {
		notFoundOr500(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	var cm *k8s.ClusterManager
	if cl.InCluster {
		cm = s.opts.Cluster
	} else {
		rc, err := k8s.NewRemoteClusterFromKubeconfigYAML(string(cl.AuthConfigEncrypted))
		if err != nil {
			writeError(w, http.StatusBadGateway, "unreachable", err.Error())
			return
		}
		cs := rc.Clientset()
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			writeError(w, http.StatusBadGateway, "unreachable", err.Error())
			return
		}
		pods, _ := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		out := buildNodeInfos(nodes.Items, pods.Items)
		writeJSON(w, http.StatusOK, out)
		return
	}

	if cm == nil {
		writeError(w, http.StatusInternalServerError, "no_cluster_manager", "cluster manager not available")
		return
	}

	cs := cm.Clientset()
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		writeError(w, http.StatusBadGateway, "unreachable", err.Error())
		return
	}

	// Get all pods across all namespaces
	pods, _ := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})

	out := buildNodeInfos(nodes.Items, pods.Items)
	writeJSON(w, http.StatusOK, out)
}

func buildNodeInfos(nodes []corev1.Node, pods []corev1.Pod) []apiv1.NodeInfo {
	// Index pods by node name
	podsByNode := make(map[string][]corev1.Pod)
	for _, p := range pods {
		if p.Spec.NodeName != "" {
			podsByNode[p.Spec.NodeName] = append(podsByNode[p.Spec.NodeName], p)
		}
	}

	out := make([]apiv1.NodeInfo, 0, len(nodes))
	for _, n := range nodes {
		info := apiv1.NodeInfo{
			Name:           n.Name,
			KubeletVersion: n.Status.NodeInfo.KubeletVersion,
			OS:             n.Status.NodeInfo.OSImage,
			Arch:           n.Status.NodeInfo.Architecture,
			CreatedAt:      n.CreationTimestamp.Time,
		}

		// Roles
		for label := range n.Labels {
			if strings.HasPrefix(label, "node-role.kubernetes.io/") {
				role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
				if role == "" {
					role = "master"
				}
				info.Roles = append(info.Roles, role)
			}
		}
		sort.Strings(info.Roles)

		// Status
		status := "NotReady"
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status == corev1.ConditionTrue {
					status = "Ready"
				}
				break
			}
		}
		info.Status = status

		// CPU
		cpuCap := n.Status.Capacity.Cpu()
		cpuAlloc := n.Status.Allocatable.Cpu()
		info.CPUCapacity = cpuCap.String()
		info.CPUAllocatable = cpuAlloc.String()

		cpuUsed := resource.NewQuantity(0, resource.DecimalSI)
		for _, p := range podsByNode[n.Name] {
			if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodPending {
				for _, c := range p.Spec.Containers {
					if req := c.Resources.Requests.Cpu(); req != nil {
						cpuUsed.Add(*req)
					}
				}
			}
		}
		info.CPUUsed = cpuUsed.String()
		if allocMilli := cpuAlloc.MilliValue(); allocMilli > 0 {
			info.CPUUsedPercent = float64(cpuUsed.MilliValue()) / float64(allocMilli) * 100
		}

		// Memory
		memCap := n.Status.Capacity.Memory()
		memAlloc := n.Status.Allocatable.Memory()
		info.MemCapacity = memCap.String()
		info.MemAllocatable = memAlloc.String()

		memUsed := resource.NewQuantity(0, resource.BinarySI)
		for _, p := range podsByNode[n.Name] {
			if p.Status.Phase == corev1.PodRunning || p.Status.Phase == corev1.PodPending {
				for _, c := range p.Spec.Containers {
					if req := c.Resources.Requests.Memory(); req != nil {
						memUsed.Add(*req)
					}
				}
			}
		}
		info.MemUsed = memUsed.String()
		if allocBytes := memAlloc.Value(); allocBytes > 0 {
			info.MemUsedPercent = float64(memUsed.Value()) / float64(allocBytes) * 100
		}

		// Pods
		nodePods := podsByNode[n.Name]
		info.PodCount = len(nodePods)
		info.Pods = make([]apiv1.PodRef, 0, len(nodePods))
		for _, p := range nodePods {
			ref := apiv1.PodRef{
				Name:      p.Name,
				Namespace: p.Namespace,
				Status:    string(p.Status.Phase),
				Health:    podHealthStatus(p),
			}
			// Owner
			for _, owner := range p.OwnerReferences {
				ref.Kind = owner.Kind
				ref.Owner = owner.Name
				break
			}
			// Resource requests (sum of containers)
			cpuReq := resource.NewQuantity(0, resource.DecimalSI)
			memReq := resource.NewQuantity(0, resource.BinarySI)
			for _, c := range p.Spec.Containers {
				if r := c.Resources.Requests.Cpu(); r != nil {
					cpuReq.Add(*r)
				}
				if r := c.Resources.Requests.Memory(); r != nil {
					memReq.Add(*r)
				}
			}
			ref.CPUReq = cpuReq.String()
			ref.MemReq = memReq.String()
			info.Pods = append(info.Pods, ref)
		}

		out = append(out, info)
	}

	// Sort: control-plane first, then by name
	sort.Slice(out, func(i, j int) bool {
		iCP := hasRole(out[i].Roles, "control-plane") || hasRole(out[i].Roles, "master")
		jCP := hasRole(out[j].Roles, "control-plane") || hasRole(out[j].Roles, "master")
		if iCP != jCP {
			return iCP
		}
		return out[i].Name < out[j].Name
	})

	return out
}

func podHealthStatus(p corev1.Pod) string {
	switch p.Status.Phase {
	case corev1.PodRunning:
		for _, c := range p.Status.ContainerStatuses {
			if c.State.Waiting != nil && c.State.Waiting.Reason != "" {
				return "Degraded"
			}
			if c.State.Terminated != nil && c.State.Terminated.ExitCode != 0 {
				return "Degraded"
			}
		}
		return "Healthy"
	case corev1.PodPending:
		return "Progressing"
	case corev1.PodFailed:
		return "Degraded"
	case corev1.PodSucceeded:
		return "Healthy"
	default:
		return "Unknown"
	}
}

func hasRole(roles []string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
