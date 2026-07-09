package service

import "context"

type httpUpstreamRequestOptionsKey struct{}

// HTTPUpstreamRequestOptions carries per-request transport overrides for the
// shared upstream HTTP client layer.
type HTTPUpstreamRequestOptions struct {
	// FreshClient bypasses the shared upstream client cache and creates a
	// one-shot http.Client/http.Transport for the current request.
	FreshClient bool
	// DisableKeepAlives disables connection reuse for the current request.
	// This implies a one-shot client so the transport can be safely mutated.
	DisableKeepAlives bool
	// RawHTTP1HeaderOrder enables one-shot raw HTTP/1.1 replay with the given
	// explicit header name order/casing. Intended for anti-ban paths where
	// net/http's canonicalization would otherwise destroy the desired wire shape.
	RawHTTP1HeaderOrder []string
}

func (o HTTPUpstreamRequestOptions) HasOverrides() bool {
	return o.FreshClient || o.DisableKeepAlives || len(o.RawHTTP1HeaderOrder) > 0
}

func (o HTTPUpstreamRequestOptions) RequiresDedicatedClient() bool {
	return o.FreshClient || o.DisableKeepAlives || len(o.RawHTTP1HeaderOrder) > 0
}

// WithHTTPUpstreamRequestOptions stores per-request upstream transport options
// on the context consumed by repository/http_upstream.go.
func WithHTTPUpstreamRequestOptions(ctx context.Context, opts HTTPUpstreamRequestOptions) context.Context {
	if ctx == nil || !opts.HasOverrides() {
		return ctx
	}
	existing := HTTPUpstreamRequestOptionsFromContext(ctx)
	if existing.HasOverrides() {
		opts.FreshClient = opts.FreshClient || existing.FreshClient
		opts.DisableKeepAlives = opts.DisableKeepAlives || existing.DisableKeepAlives
		if len(opts.RawHTTP1HeaderOrder) == 0 && len(existing.RawHTTP1HeaderOrder) > 0 {
			opts.RawHTTP1HeaderOrder = append([]string(nil), existing.RawHTTP1HeaderOrder...)
		}
	}
	return context.WithValue(ctx, httpUpstreamRequestOptionsKey{}, opts)
}

func HTTPUpstreamRequestOptionsFromContext(ctx context.Context) HTTPUpstreamRequestOptions {
	if ctx == nil {
		return HTTPUpstreamRequestOptions{}
	}
	opts, _ := ctx.Value(httpUpstreamRequestOptionsKey{}).(HTTPUpstreamRequestOptions)
	return opts
}
