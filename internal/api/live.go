package api

import (
	"net/http"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/manifest"
)

// collectLiveForAPI assembles the list of live objects matching the
// rendered desired set, using the cluster manager's informer caches.
// Returned slice is parallel to `desired` only for those entries that exist;
// missing objects are omitted (callers compute "missing" elsewhere).
func collectLiveForAPI(r *http.Request, s *Server, app *domain.Application, desired []*unstructured.Unstructured) ([]*unstructured.Unstructured, map[string]struct{}, error) {
	missing := map[string]struct{}{}
	var live []*unstructured.Unstructured
	seen := map[string]struct{}{}
	for _, d := range desired {
		mapping, err := s.opts.Cluster.MappingFor(d.GroupVersionKind())
		if err != nil {
			missing[d.GetName()] = struct{}{}
			continue
		}
		if err := s.opts.Cluster.EnsureInformer(r.Context(), mapping.Resource); err != nil {
			return nil, nil, err
		}
		ns := d.GetNamespace()
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if ns == "" {
				ns = app.DestNamespace
			}
		} else {
			ns = ""
		}
		objs, err := s.opts.Cluster.ListByLabel(mapping.Resource, ns, labels.SelectorFromSet(labels.Set{manifest.TrackingLabel: app.Name}))
		if err != nil {
			return nil, nil, err
		}
		for _, o := range objs {
			key := o.GroupVersionKind().String() + "/" + o.GetNamespace() + "/" + o.GetName()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if o.GetName() == d.GetName() {
				live = append(live, o)
			}
		}
	}
	return live, missing, nil
}
