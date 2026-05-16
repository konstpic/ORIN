package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-ui/k8s-ui/internal/k8s"
)

// NetworkMapNode is a node in the application network map.
type NetworkMapNode struct {
	UID       string            `json:"uid"`
	Group     string            `json:"group"`
	Version   string            `json:"version"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	// Selector for Services (maps labels to Pods).
	Selector map[string]string `json:"selector,omitempty"`
	// IngressBackends: service names this Ingress routes to.
	IngressBackends []string `json:"ingressBackends,omitempty"`
	// NetworkPolicy selectors and rules.
	NetPolicyPodSelector map[string]string   `json:"netPolicyPodSelector,omitempty"`
	NetPolicyIngressFrom []NetworkPolicyPeer `json:"netPolicyIngressFrom,omitempty"`
	NetPolicyEgressTo    []NetworkPolicyPeer `json:"netPolicyEgressTo,omitempty"`
}

// NetworkPolicyPeer describes a peer in a NetworkPolicy rule.
type NetworkPolicyPeer struct {
	PodSelector       map[string]string `json:"podSelector,omitempty"`
	NamespaceSelector map[string]string `json:"namespaceSelector,omitempty"`
}

// NetworkMapEdge is a directed edge between two network map nodes.
type NetworkMapEdge struct {
	SourceUID string `json:"sourceUid"`
	TargetUID string `json:"targetUid"`
	Type      string `json:"type"` // "routes", "selects", "ingress-allow", "egress-allow"
	Label     string `json:"label"`
}

// NetworkMapResponse is the response for GET /api/v1/applications/{name}/network-map.
type NetworkMapResponse struct {
	Nodes []NetworkMapNode `json:"nodes"`
	Edges []NetworkMapEdge `json:"edges"`
}

func (s *Server) getNetworkMap(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}

	ns := app.DestNamespace
	if ns == "" {
		ns = "default"
	}
	tree, err := s.opts.Cluster.BuildTree(r.Context(), app.Name, ns)
	if err != nil {
		writeError(w, http.StatusBadGateway, "build_tree_failed", err.Error())
		return
	}

	resp := buildNetworkMapResponse(tree)
	writeJSON(w, http.StatusOK, resp)
}

func buildNetworkMapResponse(tree []*k8s.ResourceNode) *NetworkMapResponse {
	// Flatten tree
	var all []*k8s.ResourceNode
	walkTree(tree, &all)

	netKinds := map[string]bool{
		"Ingress": true, "Service": true, "Pod": true,
		"ConfigMap": true, "Secret": true, "NetworkPolicy": true,
	}

	// Index nodes by UID
	nodeByUID := make(map[string]*NetworkMapNode)
	var nodes []NetworkMapNode

	for _, n := range all {
		if !netKinds[n.Object.GetKind()] {
			continue
		}
		un := n.Object
		uid := string(un.GetUID())
		if uid == "" {
			uid = un.GroupVersionKind().Group + "/" + un.GetKind() + "/" + un.GetNamespace() + "/" + un.GetName()
		}

		node := NetworkMapNode{
			UID:       uid,
			Group:     un.GroupVersionKind().Group,
			Version:   un.GroupVersionKind().Version,
			Kind:      un.GetKind(),
			Namespace: un.GetNamespace(),
			Name:      un.GetName(),
			Labels:    un.GetLabels(),
		}

		// Extract Service selector
		if un.GetKind() == "Service" {
			if sel, ok, _ := unstructured.NestedStringMap(un.Object, "spec", "selector"); ok {
				node.Selector = sel
			}
		}

		// Extract Ingress backends
		if un.GetKind() == "Ingress" {
			if rules, ok, _ := unstructured.NestedSlice(un.Object, "spec", "rules"); ok {
				for _, r := range rules {
					rm, _ := r.(map[string]interface{})
					if http, ok := rm["http"]; ok {
						httpM, _ := http.(map[string]interface{})
						if paths, ok := httpM["paths"]; ok {
							pathSlice, _ := paths.([]interface{})
							for _, p := range pathSlice {
								pm, _ := p.(map[string]interface{})
								if backend, ok := pm["backend"]; ok {
									bm, _ := backend.(map[string]interface{})
									if svc, ok := bm["service"]; ok {
										svcM, _ := svc.(map[string]interface{})
										if name, ok := svcM["name"].(string); ok && name != "" {
											node.IngressBackends = append(node.IngressBackends, name)
										}
									}
									// v1beta1 format
									if svcName, ok := bm["serviceName"].(string); ok && svcName != "" {
										node.IngressBackends = append(node.IngressBackends, svcName)
									}
								}
							}
						}
					}
				}
			}
		}

		// Extract NetworkPolicy selectors
		if un.GetKind() == "NetworkPolicy" {
			if sel, ok, _ := unstructured.NestedStringMap(un.Object, "spec", "podSelector", "matchLabels"); ok {
				node.NetPolicyPodSelector = sel
			}
			// Ingress from
			if ingressRules, ok, _ := unstructured.NestedSlice(un.Object, "spec", "ingress"); ok {
				for _, ir := range ingressRules {
					irm, _ := ir.(map[string]interface{})
					if fromSlice, ok := irm["from"]; ok {
						fromArr, _ := fromSlice.([]interface{})
						for _, f := range fromArr {
							fm, _ := f.(map[string]interface{})
							peer := NetworkPolicyPeer{}
							if ps, ok := fm["podSelector"]; ok {
								psm, _ := ps.(map[string]interface{})
								if ml, ok := psm["matchLabels"]; ok {
									mlm, _ := ml.(map[string]interface{})
									sel := make(map[string]string)
									for k, v := range mlm {
										if s, ok := v.(string); ok {
											sel[k] = s
										}
									}
									peer.PodSelector = sel
								}
							}
							if ns, ok := fm["namespaceSelector"]; ok {
								nsm, _ := ns.(map[string]interface{})
								if ml, ok := nsm["matchLabels"]; ok {
									mlm, _ := ml.(map[string]interface{})
									sel := make(map[string]string)
									for k, v := range mlm {
										if s, ok := v.(string); ok {
											sel[k] = s
										}
									}
									peer.NamespaceSelector = sel
								}
							}
							if peer.PodSelector != nil || peer.NamespaceSelector != nil {
								node.NetPolicyIngressFrom = append(node.NetPolicyIngressFrom, peer)
							}
						}
					}
				}
			}
			// Egress to
			if egressRules, ok, _ := unstructured.NestedSlice(un.Object, "spec", "egress"); ok {
				for _, er := range egressRules {
					erm, _ := er.(map[string]interface{})
					if toSlice, ok := erm["to"]; ok {
						toArr, _ := toSlice.([]interface{})
						for _, t := range toArr {
							tm, _ := t.(map[string]interface{})
							peer := NetworkPolicyPeer{}
							if ps, ok := tm["podSelector"]; ok {
								psm, _ := ps.(map[string]interface{})
								if ml, ok := psm["matchLabels"]; ok {
									mlm, _ := ml.(map[string]interface{})
									sel := make(map[string]string)
									for k, v := range mlm {
										if s, ok := v.(string); ok {
											sel[k] = s
										}
									}
									peer.PodSelector = sel
								}
							}
							if peer.PodSelector != nil {
								node.NetPolicyEgressTo = append(node.NetPolicyEgressTo, peer)
							}
						}
					}
				}
			}
		}

		nodes = append(nodes, node)
		nodeByUID[uid] = &nodes[len(nodes)-1]
	}

	// Build edges
	var edges []NetworkMapEdge
	addEdge := func(src, tgt, typ, label string) {
		for _, e := range edges {
			if e.SourceUID == src && e.TargetUID == tgt && e.Type == typ {
				return
			}
		}
		edges = append(edges, NetworkMapEdge{SourceUID: src, TargetUID: tgt, Type: typ, Label: label})
	}

	for _, node := range nodes {
		// Ingress → Service
		for _, svcName := range node.IngressBackends {
			for _, other := range nodes {
				if other.Kind == "Service" && other.Name == svcName {
					addEdge(node.UID, other.UID, "routes", "routes to")
				}
			}
		}

		// Service → Pod (via selector match)
		if node.Kind == "Service" && len(node.Selector) > 0 {
			for _, pod := range nodes {
				if pod.Kind != "Pod" || pod.Labels == nil {
					continue
				}
				if matchesSelector(pod.Labels, node.Selector) {
					addEdge(node.UID, pod.UID, "selects", "selects")
				}
			}
		}
	}

	return &NetworkMapResponse{Nodes: nodes, Edges: edges}
}

func matchesSelector(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func walkTree(nodes []*k8s.ResourceNode, out *[]*k8s.ResourceNode) {
	for _, n := range nodes {
		*out = append(*out, n)
		if len(n.Children) > 0 {
			walkTree(n.Children, out)
		}
	}
}
