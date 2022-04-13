package local

import (
	"fmt"
	"google.golang.org/grpc/resolver"
)

const (
	Scheme      = "local"
	ServiceName = "api.localhost.grpc.io"
)

func NewBuilder(addrs []string) *staticResolverBuilder {
	return &staticResolverBuilder{addrs: addrs}
}

type staticResolverBuilder struct {
	addrs []string
}

func (s *staticResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &staticResolver{
		target: target,
		cc:     cc,
		addrsStore: map[string][]string{
			ServiceName: s.addrs,
		},
	}
	r.start()
	return r, nil
}

func (s *staticResolverBuilder) Scheme() string {
	return Scheme
}

func (s *staticResolverBuilder) Target() string {
	return fmt.Sprintf("%s:///%s", Scheme, ServiceName)
}

type staticResolver struct {
	target     resolver.Target
	cc         resolver.ClientConn
	addrsStore map[string][]string
}

func (r *staticResolver) start() {
	addrStrs := r.addrsStore[r.target.Endpoint]
	addrs := make([]resolver.Address, len(addrStrs))
	for i, s := range addrStrs {
		addrs[i] = resolver.Address{Addr: s}
	}
	r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (*staticResolver) ResolveNow(o resolver.ResolveNowOptions) {}

func (*staticResolver) Close() {}
