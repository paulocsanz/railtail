package main

import (
	"context"
	"fmt"
	"io"
	"net"

	"golang.org/x/sync/errgroup"
)

// Reversed from upstream: upstream forwards a Railway-network-facing
// listener out to a tailnet target (net.Listen + ts.Dial). We need the
// opposite — a tailnet-facing listener forwarding to a Railway-private
// target (ts.Listen + plain net.Dial) — see main.go.
func fwdTCP(lstConn net.Conn, targetAddr string) error {
	defer lstConn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dialer net.Dialer
	targetConn, err := dialer.DialContext(ctx, "tcp", targetAddr)
	if err != nil {
		return fmt.Errorf("failed to dial target: %w", err)
	}

	defer targetConn.Close()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if _, err := io.Copy(targetConn, lstConn); err != nil {
			return fmt.Errorf("failed to copy data to target: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		if _, err := io.Copy(lstConn, targetConn); err != nil {
			return fmt.Errorf("failed to copy data from target: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("connection error: %w", err)
	}

	return nil
}
