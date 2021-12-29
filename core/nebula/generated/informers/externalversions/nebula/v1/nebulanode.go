// gen

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	"context"
	time "time"

	nebulav1 "github.com/giolekva/pcloud/core/nebula/apis/nebula/v1"
	versioned "github.com/giolekva/pcloud/core/nebula/generated/clientset/versioned"
	internalinterfaces "github.com/giolekva/pcloud/core/nebula/generated/informers/externalversions/internalinterfaces"
	v1 "github.com/giolekva/pcloud/core/nebula/generated/listers/nebula/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// NebulaNodeInformer provides access to a shared informer and lister for
// NebulaNodes.
type NebulaNodeInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.NebulaNodeLister
}

type nebulaNodeInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewNebulaNodeInformer constructs a new informer for NebulaNode type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewNebulaNodeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredNebulaNodeInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredNebulaNodeInformer constructs a new informer for NebulaNode type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredNebulaNodeInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LekvaV1().NebulaNodes(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.LekvaV1().NebulaNodes(namespace).Watch(context.TODO(), options)
			},
		},
		&nebulav1.NebulaNode{},
		resyncPeriod,
		indexers,
	)
}

func (f *nebulaNodeInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredNebulaNodeInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *nebulaNodeInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&nebulav1.NebulaNode{}, f.defaultInformer)
}

func (f *nebulaNodeInformer) Lister() v1.NebulaNodeLister {
	return v1.NewNebulaNodeLister(f.Informer().GetIndexer())
}