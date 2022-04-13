package request

import (
	"context"

	"github.com/Shanghai-Lunara/pkg/zaplogger"
	"google.golang.org/grpc"
	grpcresolver "google.golang.org/grpc/resolver"

	"github.com/nevercase/grpc-k8s-resolver/pkg/env"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver/k8s"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver/local"
)

type Option struct {
	Resolver *Resolver
}

type Resolver struct {
	// StaticAddress and DynamicOption could only be specified one at the same time
	StaticAddress []string
	DynamicOption *k8s.BuilderOption
}

type Client struct {
	hostname string
	cc       *grpc.ClientConn
}

func NewClient(opt *Option) *Client {
	if opt.Resolver == nil {
		zaplogger.Sugar().Fatal("Resolver must be specified")
	}
	var builder resolver.Builder
	if len(opt.Resolver.StaticAddress) > 0 {
		builder = local.NewBuilder(opt.Resolver.StaticAddress)
	} else {
		if opt.Resolver.DynamicOption == nil {
			zaplogger.Sugar().Fatal("Resolver StaticAddress or DynamicOption must be specified one")
		}
		builder = k8s.NewBuilder(context.Background(), opt.Resolver.DynamicOption)
	}
	grpcresolver.Register(builder)
	roundrobinConn, err := grpc.Dial(
		builder.Target(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
		grpc.WithInsecure(),
		grpc.WithBlock(),
	)
	if err != nil {
		zaplogger.Sugar().Fatal(err)
	}
	c := &Client{
		hostname: env.GetHostNameMustSpecified(),
		cc:       roundrobinConn,
	}
	return c
}
