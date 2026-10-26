package discovery

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ErrNotSynced is Ready's answer until every informer has listed once.
var ErrNotSynced = errors.New("the discovery has not listed Ingresses and EndpointSlices yet")

// serviceIndex finds a namespace's EndpointSlices by the Service they
// belong to: namespace/service.
const serviceIndex = "service"

// Discovery watches Ingresses and EndpointSlices — Namespaces too for a
// label selector — and answers the apps they describe (docs/adr/0007 D5).
// An Ingress is read when it changes, and what it says is kept; its status
// is counted from the EndpointSlice cache when the apps are asked for.
type Discovery struct {
	opts      Options
	logger    *slog.Logger
	factories []informers.SharedInformerFactory
	// slices are the EndpointSlice caches by namespace; "" is the one cache
	// of every namespace.
	slices map[string]cache.Indexer
	// namespaces is the Namespace cache, for a label selector only.
	namespaces cache.Indexer

	mu   sync.RWMutex
	apps map[string]App

	synced atomic.Bool
}

// New sets the informers up as the namespace selector needs them: one set
// over every namespace, or one set per named namespace, which the chart's
// namespaced Roles allow. It reaches the API server only once Run starts.
func New(client kubernetes.Interface, opts Options, logger *slog.Logger) (*Discovery, error) {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	d := &Discovery{opts: opts, logger: logger, slices: map[string]cache.Indexer{}, apps: map[string]App{}}
	scopes := []string{metav1.NamespaceAll}
	if opts.Namespaces.mode() == perNamespace {
		scopes = opts.Namespaces.Names
	}
	for _, ns := range scopes {
		f := informers.NewSharedInformerFactoryWithOptions(client, 0, informers.WithNamespace(ns), informers.WithTransform(trim))
		ingresses := f.Networking().V1().Ingresses().Informer()
		endpointSlices := f.Discovery().V1().EndpointSlices().Informer()
		if err := endpointSlices.AddIndexers(cache.Indexers{serviceIndex: byService}); err != nil {
			return nil, err
		}
		if _, err := ingresses.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    d.upsert,
			UpdateFunc: func(_, obj any) { d.upsert(obj) },
			DeleteFunc: d.remove,
		}); err != nil {
			return nil, err
		}
		if err := d.watchErrors(ingresses, "ingresses", ns); err != nil {
			return nil, err
		}
		if err := d.watchErrors(endpointSlices, "endpointslices", ns); err != nil {
			return nil, err
		}
		d.slices[ns] = endpointSlices.GetIndexer()
		d.factories = append(d.factories, f)
	}
	if opts.Namespaces.mode() == byLabels {
		namespaces := d.factories[0].Core().V1().Namespaces().Informer()
		if err := d.watchErrors(namespaces, "namespaces", ""); err != nil {
			return nil, err
		}
		d.namespaces = namespaces.GetIndexer()
	}
	return d, nil
}

// byService indexes an EndpointSlice by its Service, which the
// EndpointSlice controller names in the label kubernetes.io/service-name.
func byService(obj any) ([]string, error) {
	slice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		return nil, nil
	}
	svc := slice.Labels[discoveryv1.LabelServiceName]
	if svc == "" {
		return nil, nil
	}
	return []string{slice.Namespace + "/" + svc}, nil
}

// watchErrors logs a failed list or watch as a warning — a missing RBAC
// grant shows here as "forbidden" — and the expected ends of a watch not
// at all, as client-go's own handler does.
func (d *Discovery) watchErrors(informer cache.SharedIndexInformer, resource, namespace string) error {
	return informer.SetWatchErrorHandlerWithContext(func(_ context.Context, _ *cache.Reflector, err error) {
		switch {
		case errors.Is(err, io.EOF):
		case apierrors.IsResourceExpired(err) || apierrors.IsGone(err):
			d.logger.Debug("the watch restarts", "resource", resource, "namespace", namespace, "error", err)
		default:
			d.logger.Warn("the list or watch failed", "resource", resource, "namespace", namespace, "error", err)
		}
	})
}

// Run starts the informers, marks the discovery ready once each has listed,
// and stops them when ctx ends.
func (d *Discovery) Run(ctx context.Context) {
	for _, f := range d.factories {
		f.Start(ctx.Done())
	}
	defer func() {
		for _, f := range d.factories {
			f.Shutdown()
		}
	}()
	for _, f := range d.factories {
		for _, ok := range f.WaitForCacheSync(ctx.Done()) {
			if !ok {
				return // ctx ended first
			}
		}
	}
	d.synced.Store(true)
	d.logger.Info("the discovery listed every Ingress", "apps", len(d.Apps()))
	<-ctx.Done()
}

// Ready is nil once every informer has listed, ErrNotSynced before. Like the
// probe of the API server (internal/kube), it stays ready once it was.
func (d *Discovery) Ready(context.Context) error {
	if !d.synced.Load() {
		return ErrNotSynced
	}
	return nil
}

// upsert reads an Ingress that was added or changed.
func (d *Discovery) upsert(obj any) {
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return
	}
	app, why, ignored := appOf(ing, d.opts.Filter)
	// An Ingress of a namespace the selector does not take is no business
	// of the log; with a label selector it is read all the same, in case
	// its namespace's labels change.
	logged := d.selected(ing.Namespace)
	for _, k := range ignored {
		if logged {
			d.logger.Warn("an annotation is ignored", "namespace", ing.Namespace, "ingress", ing.Name, "key", k.key, "reason", k.reason)
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if why != nil {
		delete(d.apps, ing.Namespace+"/"+ing.Name)
		if logged && why.notable {
			d.logger.Warn("an Ingress is no app", "namespace", ing.Namespace, "ingress", ing.Name, "reason", why.reason)
		} else if logged {
			d.logger.Debug("an Ingress is no app", "namespace", ing.Namespace, "ingress", ing.Name, "reason", why.reason)
		}
		return
	}
	d.apps[app.Key()] = app
}

// remove forgets a deleted Ingress.
func (d *Discovery) remove(obj any) {
	if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tomb.Obj
	}
	ing, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.apps, ing.Namespace+"/"+ing.Name)
}

// selected reports whether the namespace is selected now. Only a label
// selector needs to ask: the other layouts watch the selected namespaces
// alone.
func (d *Discovery) selected(namespace string) bool {
	if d.namespaces == nil {
		return true
	}
	var nsLabels map[string]string
	if obj, ok, _ := d.namespaces.GetByKey(namespace); ok {
		if ns, isNS := obj.(*corev1.Namespace); isNS {
			nsLabels = ns.Labels
		}
	}
	return d.opts.Namespaces.selects(namespace, nsLabels)
}

// Apps are the apps of the selected namespaces, each with its status as the
// EndpointSlice cache has it now, ordered by namespace and Ingress name.
func (d *Discovery) Apps() []App {
	d.mu.RLock()
	apps := make([]App, 0, len(d.apps))
	for _, a := range d.apps {
		apps = append(apps, a)
	}
	d.mu.RUnlock()

	apps = slices.DeleteFunc(apps, func(a App) bool { return !d.selected(a.Namespace) })
	for i, a := range apps {
		apps[i].Status = Status{State: Unknown}
		if a.StatusCheck {
			apps[i].Status = statusOf(d.endpointSlices(a.Namespace, a.Services))
		}
	}
	slices.SortFunc(apps, func(a, b App) int { return strings.Compare(a.Key(), b.Key()) })
	return apps
}

// endpointSlices are the cached EndpointSlices of a namespace's Services.
func (d *Discovery) endpointSlices(namespace string, services []string) []*discoveryv1.EndpointSlice {
	idx, ok := d.slices[namespace]
	if !ok {
		idx = d.slices[metav1.NamespaceAll]
	}
	if idx == nil {
		return nil
	}
	var out []*discoveryv1.EndpointSlice
	for _, svc := range services {
		objs, err := idx.ByIndex(serviceIndex, namespace+"/"+svc)
		if err != nil {
			continue
		}
		for _, obj := range objs {
			if slice, isSlice := obj.(*discoveryv1.EndpointSlice); isSlice {
				out = append(out, slice)
			}
		}
	}
	return out
}
