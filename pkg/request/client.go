package request

import (
	"context"

	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	grpcresolver "google.golang.org/grpc/resolver"

	"github.com/Shanghai-Lunara/pkg/zaplogger"

	"github.com/nevercase/grpc-k8s-resolver/pkg/env"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver/k8s"
	"github.com/nevercase/grpc-k8s-resolver/pkg/resolver/local"
)

type Option struct {
	Resolver   *Resolver
	Secret     string
	CertFile   string
	ServerName string
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
	// Set up the credentials for the connection.
	perRPC := oauth.NewOauthAccess(fetchToken(opt.Secret))
	creds, err := credentials.NewClientTLSFromFile(opt.CertFile, opt.ServerName)
	if err != nil {
		zaplogger.Sugar().Fatalf("failed to load credentials: %v", err)
	}
	roundrobinConn, err := grpc.Dial(
		builder.Target(),
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig": [{"round_robin":{}}]}`),
		grpc.WithBlock(),
		// In addition to the following grpc.DialOption, callers may also use
		// the grpc.CallOption grpc.PerRPCCredentials with the RPC invocation
		// itself.
		// See: https://godoc.org/google.golang.org/grpc#PerRPCCredentials
		grpc.WithPerRPCCredentials(perRPC),
		// oauth.NewOauthAccess requires the configuration of transport
		// credentials.
		grpc.WithTransportCredentials(creds),
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

// fetchToken simulates a token lookup and omits the details of proper token
// acquisition. For examples of how to acquire an OAuth2 token, see:
// https://godoc.org/golang.org/x/oauth2
func fetchToken(secret string) *oauth2.Token {
	return &oauth2.Token{
		AccessToken: secret,
	}
}
