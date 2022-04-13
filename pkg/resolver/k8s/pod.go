package k8s

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Shanghai-Lunara/pkg/zaplogger"
	"go.uber.org/zap"
	"google.golang.org/grpc/resolver"


	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	Scheme      = "kubernetes"
	ServiceName = "api.kubernetes.grpc.io"
	PortName    = "apigrpc"
)

type BuilderOption struct {
	ClientSet kubernetes.Interface
	Namespace string
	Labels    map[string]string
}

func (b *BuilderOption) Validate() error {
	if b.Namespace == "" {
		return fmt.Errorf("BuilderOption Namespace must be specified and no empty")
	}
	return nil
}

func NewBuilder(ctx context.Context, opt *BuilderOption) *dynamicResolverBuilder {
	return &dynamicResolverBuilder{
		ctx:           ctx,
		builderOption: opt,
	}
}

type dynamicResolverBuilder struct {
	ctx           context.Context
	builderOption *BuilderOption
}

func (s *dynamicResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &staticResolver{
		target:        target,
		cc:            cc,
		builderOption: s.builderOption,
		pods:          make(map[string]*corev1.Pod),
		ctx:           s.ctx,
	}
	go r.watcher()
	return r, nil
}

func (s *dynamicResolverBuilder) Scheme() string {
	return Scheme
}

func (s *dynamicResolverBuilder) Target() string {
	return fmt.Sprintf("%s:///%s", Scheme, ServiceName)
}

type staticResolver struct {
	target        resolver.Target
	cc            resolver.ClientConn
	builderOption *BuilderOption
	pods          map[string]*corev1.Pod
	ctx           context.Context
	cancel        context.CancelFunc
}

// GetLabelSelector returns the LabelSelector of the metav1.ListOptions
func GetLabelSelector(in map[string]string) string {
	ls := labels.NewSelector()
	for k, v := range in {
		req, err := labels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			zaplogger.Sugar().Fatal(err)
		}
		ls = ls.Add(*req)
	}
	return ls.String()
}

func (r *staticResolver) watcher() {
	timeout := int64(3600 * 24)
	var opts = metav1.ListOptions{
		LabelSelector:  GetLabelSelector(r.builderOption.Labels),
		TimeoutSeconds: &timeout,
	}
rewatch:
	res, err := r.builderOption.ClientSet.CoreV1().Pods(r.builderOption.Namespace).Watch(r.ctx, opts)
	if err != nil {
		zaplogger.Sugar().Error(err)
		<-time.After(time.Second * 2)
		goto rewatch
	}
	defer res.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case e, isClosed := <-res.ResultChan():
			zaplogger.Sugar().Debugf("watch e:%#v", e)
			zaplogger.Sugar().Debugf("watch Object:%#v", e.Object)
			if !isClosed {
				goto rewatch
			}
			if err := r.handleEvent(e); err != nil {
				zaplogger.Sugar().Errorw("staticResolver watcher handleEvent error",
					zap.String("namespace", r.builderOption.Namespace),
					zap.String("labels", opts.LabelSelector),
					zap.Error(err))
				res.Stop()
				goto rewatch
			}
			r.updateState()
		}
	}
}

func (r *staticResolver) handleEvent(e watch.Event) error {
	switch e.Type {
	case watch.Modified, watch.Added:
		pod := e.Object.(*corev1.Pod)
		if t, ok := r.pods[pod.Name]; ok {
			if t.ResourceVersion > pod.ResourceVersion {
				return nil
			}
		}
		r.pods[pod.Name] = pod
	case watch.Deleted:
		pod := e.Object.(*corev1.Pod)
		if t, ok := r.pods[pod.Name]; ok {
			if t.ResourceVersion > pod.ResourceVersion {
				return nil
			}
		}
		delete(r.pods, pod.Name)
	case watch.Error:
		return fmt.Errorf("watch receive ERROR event obj: %#v", e.Object)
	}
	return nil
}

func (r *staticResolver) updateState() {
	l := make([]*corev1.Pod, 0)
	for _, v := range r.pods {
		if v.Status.Phase != corev1.PodRunning {
			continue
		}
		if len(v.Spec.Containers) != 1 {
			continue
		}
		if len(v.Spec.Containers[0].Ports) == 0 {
			continue
		}
		for _, port := range v.Spec.Containers[0].Ports {
			if port.Name == PortName {
				l = append(l, v)
			}
		}
	}
	sort.Slice(l, func(i, j int) bool {
		return l[i].CreationTimestamp.Sub(l[j].CreationTimestamp.Time).Seconds() <= 0
	})
	addrs := make([]resolver.Address, len(l))
	for _, v := range l {
		var port int32
		for _, v2 := range v.Spec.Containers[0].Ports {
			if v2.Name == PortName {
				port = v2.ContainerPort
			}
		}
		addrs = append(addrs, resolver.Address{
			Addr:       fmt.Sprintf("%s:%d", v.Status.PodIP, port),
			ServerName: ServiceName,
		})
	}
	zaplogger.Sugar().Debugf("updateState addrs:%#v", addrs)
	r.cc.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: nil,
		Attributes:    nil,
	})
}

func (*staticResolver) ResolveNow(o resolver.ResolveNowOptions) {}

func (*staticResolver) Close() {}
