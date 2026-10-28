// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/errs"

	"github.com/otfabric/iec61850ctl/pkg/network"

	"github.com/spf13/cobra"
)

var (
	discoverHost       string
	discoverPort       int
	discoverResolveMac bool
)

var discoverCmd = &cobra.Command{
	Use:     "discover",
	Aliases: []string{"scan"},
	Short:   "Discover IEC 61850 devices on the network",
	Long: `Scan a network subnet or IP range for IEC 61850 devices.

Performs TCP port scanning and validates IEC 61850 connectivity.
Supports CIDR notation (10.0.0.0/24) and IP ranges (10.0.0.5-25).

Examples:
  iec61850ctl discover --host 192.168.1.0/24
  iec61850ctl discover --host 10.0.0.50-60 --port 102
  iec61850ctl scan --host 192.168.1.0/24 --resolve-mac

Note: --resolve-mac requires root/administrator privileges to access ARP cache.`,
	RunE: runDiscover,
}

func runDiscover(cmd *cobra.Command, args []string) error {
	if discoverHost == "" {
		return errs.Invalidf("--host is required (e.g., 10.0.0.0/24 or 10.0.0.5-25)")
	}

	// Warn about MAC resolution privileges
	if discoverResolveMac {
		if os.Geteuid() != 0 {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Warning: --resolve-mac requires root privileges. MAC resolution may fail.\n")
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "   Run with: sudo %s\n\n", os.Args[0])
		}
	}

	// Parse host specification
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Parsing host specification: %s\n", discoverHost)
	ips, err := network.ParseHostSpec(discoverHost)
	if err != nil {
		return errs.Mark(fmt.Errorf("failed to parse host specification: %w", err), errs.ErrInvalidInput)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Scanning %d IP address(es) on port %d...\n\n", len(ips), discoverPort)

	// Scan in parallel
	targets := scanTargets(cmd, ips, discoverPort, discoverResolveMac)

	// Display results
	displayResults(cmd.OutOrStdout(), targets)

	return nil
}

// scanTargets performs parallel scanning of all target IPs.
func scanTargets(cmd *cobra.Command, ips []string, port int, resolveMac bool) []network.Target {
	out := cmd.OutOrStdout()
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		targets []network.Target
	)

	// Limit concurrent scans to avoid overwhelming the network
	semaphore := make(chan struct{}, 20)

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			target := scanTarget(cmd.Context(), ip, port, resolveMac)

			mu.Lock()
			defer mu.Unlock()
			targets = append(targets, target)

			// Print progress
			if target.TCPReachable {
				_, _ = fmt.Fprintf(out, "✓ %s:%d - TCP open", ip, port)
				if target.IEC61850OK {
					_, _ = fmt.Fprintf(out, " (IEC 61850 OK)")
				} else {
					_, _ = fmt.Fprintf(out, " (IEC 61850 FAILED: %s)", target.Error)
				}
				if resolveMac && target.MACAddress != "" {
					_, _ = fmt.Fprintf(out, " [MAC: %s]", target.MACAddress)
				}
				_, _ = fmt.Fprintln(out)
			}
		}(ip)
	}

	wg.Wait()
	sort.Slice(targets, func(i, j int) bool { return lessIP(targets[i].IP, targets[j].IP) })
	return targets
}

// scanTarget scans a single target IP.
func scanTarget(ctx context.Context, ip string, port int, resolveMac bool) network.Target {
	target := network.Target{
		IP:   ip,
		Port: port,
	}

	// TCP port scan
	tcpTimeout := 2 * time.Second
	target.TCPReachable = network.ScanTCP(ip, port, tcpTimeout)

	if !target.TCPReachable {
		return target
	}

	// Test IEC 61850 connection via shared session helper.
	session, err := openClientSessionTo(ctx, nil, ip, port, clientSessionOptions{
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 5 * time.Second,
		Quiet:          true,
	})
	if err != nil {
		target.IEC61850OK = false
		target.Error = err.Error()
	} else {
		target.IEC61850OK = true
		session.Close()
	}

	// Resolve MAC address if requested
	if resolveMac {
		mac, err := network.GetMACAddress(ip)
		if err == nil {
			target.MACAddress = mac
		}
	}

	return target
}

// displayResults prints a summary table of scan results.
func displayResults(out io.Writer, targets []network.Target) {
	if len(targets) == 0 {
		return
	}
	var (
		tcpOpen     int
		iec61850OK  int
		macResolved int
	)

	_, _ = fmt.Fprintln(out, "\n"+"═══════════════════════════════════════════════════════════")
	_, _ = fmt.Fprintln(out, "Scan Results")
	_, _ = fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")

	for _, t := range targets {
		if t.TCPReachable {
			tcpOpen++
		}
		if t.IEC61850OK {
			iec61850OK++
		}
		if t.MACAddress != "" {
			macResolved++
		}
	}

	_, _ = fmt.Fprintf(out, "\nTotal IPs scanned:        %d\n", len(targets))
	_, _ = fmt.Fprintf(out, "TCP port %d open:        %d\n", targets[0].Port, tcpOpen)
	_, _ = fmt.Fprintf(out, "IEC 61850 devices found:  %d\n", iec61850OK)
	if discoverResolveMac {
		_, _ = fmt.Fprintf(out, "MAC addresses resolved:   %d\n", macResolved)
	}

	if iec61850OK > 0 {
		_, _ = fmt.Fprintln(out, "\n"+"IEC 61850 Devices:")
		_, _ = fmt.Fprintln(out, "───────────────────────────────────────────────────────────")
		for _, t := range targets {
			if t.IEC61850OK {
				_, _ = fmt.Fprintf(out, "  %s:%d", t.IP, t.Port)
				if t.MACAddress != "" {
					_, _ = fmt.Fprintf(out, "  [MAC: %s]", t.MACAddress)
				}
				_, _ = fmt.Fprintln(out)
			}
		}
	}

	if tcpOpen > iec61850OK {
		_, _ = fmt.Fprintln(out, "\n"+"TCP Open but IEC 61850 Failed:")
		_, _ = fmt.Fprintln(out, "───────────────────────────────────────────────────────────")
		for _, t := range targets {
			if t.TCPReachable && !t.IEC61850OK {
				_, _ = fmt.Fprintf(out, "  %s:%d - %s\n", t.IP, t.Port, t.Error)
			}
		}
	}
}

func init() {
	discoverCmd.Flags().StringVar(&discoverHost, "host", "", "subnet (10.0.0.0/24) or IP range (10.0.0.5-25) to scan (required)")
	discoverCmd.Flags().IntVar(&discoverPort, "port", 102, "port to scan")
	discoverCmd.Flags().BoolVar(&discoverResolveMac, "resolve-mac", false, "resolve MAC addresses (requires root privileges)")

	_ = discoverCmd.MarkFlagRequired("host")

	rootCmd.AddCommand(discoverCmd)
}

// lessIP orders addresses numerically (falls back to string order).
func lessIP(a, b string) bool {
	pa, errA := netip.ParseAddr(a)
	pb, errB := netip.ParseAddr(b)
	if errA != nil || errB != nil {
		return a < b
	}
	return pa.Less(pb)
}
