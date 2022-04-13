package resolver

import "google.golang.org/grpc/resolver"

// Builder creates a resolver that will be used to watch name resolution updates.
// Builder implements the resolver.Builder
type Builder interface {
	// Build creates a new resolver for the given target.
	//
	// gRPC dial calls Build synchronously, and fails if the returned error is
	// not nil.
	Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error)
	// Scheme returns the scheme supported by this resolver.
	// Scheme is defined at https://github.com/grpc/grpc/blob/master/doc/naming.md.
	Scheme() string
	// Target returns the target endpoint of this resolver.
	Target() string
}
