package limiter

import (
	"context"
	"net"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type bucket struct {
	ch chan struct{}
}

func newBucket(n int) *bucket {
	if n <= 0 {
		n = 1 // минимальная ёмкость
	}
	return &bucket{ch: make(chan struct{}, n)}
}

func (b *bucket) acquireWithContext(ctx context.Context) error {
	select {
	case b.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *bucket) release() { <-b.ch }

type perClientLimits struct {
	mu      sync.RWMutex
	buckets map[string]*bucket // key is category: "io" or "list"
}

type Limiter struct {
	mu      sync.RWMutex
	clients map[string]*perClientLimits // key is client address (ip)
	ioMax   int
	listMax int
}

func New(ioMax, listMax int) *Limiter {
	if ioMax <= 0 {
		ioMax = 1
	}
	if listMax <= 0 {
		listMax = 1
	}
	return &Limiter{
		clients: make(map[string]*perClientLimits),
		ioMax:   ioMax,
		listMax: listMax,
	}
}

func (l *Limiter) getClient(addr string) *perClientLimits {
	l.mu.Lock()
	defer l.mu.Unlock()
	pcl, ok := l.clients[addr]
	if !ok {
		pcl = &perClientLimits{buckets: make(map[string]*bucket)}
		l.clients[addr] = pcl
	}
	return pcl
}

func (p *perClientLimits) getBucket(category string, size int) *bucket {
	p.mu.Lock()
	defer p.mu.Unlock()
	b, ok := p.buckets[category]
	if !ok {
		b = newBucket(size)
		p.buckets[category] = b
	}
	return b
}

func (l *Limiter) classify(fullMethod string) (category string, size int) {
	// fullMethod example: "/filesvc.v1.FileService/Upload"
	if strings.HasSuffix(fullMethod, "/ListFiles") {
		return "list", l.listMax
	}
	// Upload and Download fall here
	return "io", l.ioMax
}

// normalizeIP extracts IP address from IP:port string
func normalizeIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If it's not in IP:port format, assume it's already an IP
		return addr
	}
	return host
}

func (l *Limiter) acquire(ctx context.Context, fullMethod string) (func(), error) {
	addr := "unknown"
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		addr = normalizeIP(p.Addr.String())
	}
	cat, size := l.classify(fullMethod)
	pcl := l.getClient(addr)
	bucket := pcl.getBucket(cat, size)

	if err := bucket.acquireWithContext(ctx); err != nil {
		return nil, err
	}

	return func() { bucket.release() }, nil
}

func (l *Limiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		release, err := l.acquire(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		defer release()
		return handler(ctx, req)
	}
}

func (l *Limiter) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		release, err := l.acquire(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		defer release()
		return handler(srv, ss)
	}
}
