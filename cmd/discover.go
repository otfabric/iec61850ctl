// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/otfabric/iec61850ctl/pkg/network"
	"github.com/otfabric/iec61850ctl/pkg/stack/client"

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
		return fmt.Errorf("--host is required (e.g., 10.0.0.0/24 or 10.0.0.5-25)")
	}

	// Warn about MAC resolution privileges
	if discoverResolveMac {
		if os.Geteuid() != 0 {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: --resolve-mac requires root privileges. MAC resolution may fail.\n")
			_, _ = fmt.Fprintf(os.Stderr, "   Run with: sudo %s\n\n", os.Args[0])
		}
	}

	// Parse host specification
	fmt.Printf("Parsing host specification: %s\n", discoverHost)
	ips, err := network.ParseHostSpec(discoverHost)
	if err != nil {
		return fmt.Errorf("failed to parse host specification: %w", err)
	}

	fmt.Printf("Scanning %d IP address(es) on port %d...\n\n", len(ips), discoverPort)

	// Scan in parallel
	targets := scanTargets(ips, discoverPort, discoverResolveMac)

	// Display results
	displayResults(targets)

	return nil
}

// scanTargets performs parallel scanning of all target IPs.
func scanTargets(ips []string, port int, resolveMac bool) []network.Target {
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

			target := scanTarget(ip, port, resolveMac)

			mu.Lock()
			targets = append(targets, target)
			mu.Unlock()

			// Print progress
			if target.TCPReachable {
				fmt.Printf("✓ %s:%d - TCP open", ip, port)
				if target.IEC61850OK {
					fmt.Printf(" (IEC 61850 OK)")
				} else {
					fmt.Printf(" (IEC 61850 FAILED: %s)", target.Error)
				}
				if resolveMac && target.MACAddress != "" {
					fmt.Printf(" [MAC: %s]", target.MACAddress)
				}
				fmt.Println()
			}
		}(ip)
	}

	wg.Wait()
	return targets
}

// scanTarget scans a single target IP.
func scanTarget(ip string, port int, resolveMac bool) network.Target {
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

	// Test IEC 61850 connection
	conn, err := client.NewConnection(client.ConnectionInput{
		Host:           ip,
		Port:           port,
		ConnectTimeout: 5,
		RequestTimeout: 5,
	})

	if err != nil {
		target.IEC61850OK = false
		target.Error = err.Error()
	} else {
		target.IEC61850OK = true
		_ = conn.Close(context.Background())
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
func displayResults(targets []network.Target) {
	var (
		tcpOpen     int
		iec61850OK  int
		macResolved int
	)

	fmt.Println("\n" + "═══════════════════════════════════════════════════════════")
	fmt.Println("Scan Results")
	fmt.Println("═══════════════════════════════════════════════════════════")

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

	fmt.Printf("\nTotal IPs scanned:        %d\n", len(targets))
	fmt.Printf("TCP port %d open:        %d\n", targets[0].Port, tcpOpen)
	fmt.Printf("IEC 61850 devices found:  %d\n", iec61850OK)
	if discoverResolveMac {
		fmt.Printf("MAC addresses resolved:   %d\n", macResolved)
	}

	if iec61850OK > 0 {
		fmt.Println("\n" + "IEC 61850 Devices:")
		fmt.Println("───────────────────────────────────────────────────────────")
		for _, t := range targets {
			if t.IEC61850OK {
				fmt.Printf("  %s:%d", t.IP, t.Port)
				if t.MACAddress != "" {
					fmt.Printf("  [MAC: %s]", t.MACAddress)
				}
				fmt.Println()
			}
		}
	}

	if tcpOpen > iec61850OK {
		fmt.Println("\n" + "TCP Open but IEC 61850 Failed:")
		fmt.Println("───────────────────────────────────────────────────────────")
		for _, t := range targets {
			if t.TCPReachable && !t.IEC61850OK {
				fmt.Printf("  %s:%d - %s\n", t.IP, t.Port, t.Error)
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
