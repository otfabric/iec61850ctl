// SPDX-License-Identifier: MIT

// Package network provides network scanning and discovery utilities
// for IEC 61850 devices.
package network

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Target represents a discovered IEC 61850 target.
type Target struct {
	IP           string
	Port         int
	TCPReachable bool
	IEC61850OK   bool
	MACAddress   string
	Error        string
}

// ParseHostSpec parses a host specification into a list of IP addresses.
// Supports:
//   - CIDR notation: 10.0.0.0/24
//   - IP range: 10.0.0.5-25
//   - Single IP: 10.0.0.5
func ParseHostSpec(hostSpec string) ([]string, error) {
	// Check for CIDR notation
	if strings.Contains(hostSpec, "/") {
		return parseCIDR(hostSpec)
	}

	// Check for range notation
	if strings.Contains(hostSpec, "-") {
		return parseRange(hostSpec)
	}

	// Single IP
	ip := net.ParseIP(hostSpec)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", hostSpec)
	}
	return []string{hostSpec}, nil
}

// parseCIDR parses CIDR notation and returns all IPs in the subnet.
func parseCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR notation: %w", err)
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast addresses for IPv4
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}

	return ips, nil
}

// parseRange parses IP range notation (e.g., 10.0.0.5-25).
func parseRange(rangeSpec string) ([]string, error) {
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format: %s (expected format: 10.0.0.5-25)", rangeSpec)
	}

	startIP := net.ParseIP(parts[0])
	if startIP == nil {
		return nil, fmt.Errorf("invalid start IP: %s", parts[0])
	}

	// Determine if end is a full IP or just the last octet
	var endIP net.IP
	if strings.Contains(parts[1], ".") {
		endIP = net.ParseIP(parts[1])
		if endIP == nil {
			return nil, fmt.Errorf("invalid end IP: %s", parts[1])
		}
	} else {
		// Just last octet - build full IP
		lastOctet, err := strconv.Atoi(parts[1])
		if err != nil || lastOctet < 0 || lastOctet > 255 {
			return nil, fmt.Errorf("invalid end octet: %s (must be 0-255)", parts[1])
		}

		// Parse start IP to get first 3 octets
		startIPv4 := startIP.To4()
		if startIPv4 == nil {
			return nil, fmt.Errorf("range notation only supports IPv4")
		}

		endIP = net.IPv4(startIPv4[0], startIPv4[1], startIPv4[2], byte(lastOctet))
	}

	// Generate IPs from start to end
	var ips []string
	for ip := make(net.IP, len(startIP)); ; inc(ip) {
		copy(ip, startIP)
		ips = append(ips, ip.String())

		if ip.Equal(endIP) {
			break
		}
		inc(startIP)

		// Sanity check to prevent infinite loops
		if len(ips) > 256 {
			return nil, fmt.Errorf("range too large (max 256 addresses)")
		}
	}

	return ips, nil
}

// inc increments an IP address.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ScanTCP performs a TCP port scan on the given host and port.
func ScanTCP(host string, port int, timeout time.Duration) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// GetMACAddress attempts to resolve the MAC address for an IP using ARP.
// This may require root/admin privileges depending on the platform.
// Returns empty string and error if resolution fails.
func GetMACAddress(ip string) (string, error) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin": // macOS
		cmd = exec.Command("arp", "-n", ip)
	case "linux":
		cmd = exec.Command("arp", "-n", ip)
	case "windows":
		cmd = exec.Command("arp", "-a", ip)
	default:
		return "", fmt.Errorf("MAC resolution not supported on %s", runtime.GOOS)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute arp command: %w", err)
	}

	// Parse ARP output to extract MAC address
	mac := parseARPOutput(string(output), runtime.GOOS)
	if mac == "" {
		return "", fmt.Errorf("MAC address not found in ARP cache for %s", ip)
	}

	return mac, nil
}

// parseARPOutput extracts MAC address from ARP command output.
func parseARPOutput(output, platform string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch platform {
		case "darwin", "linux":
			// macOS/Linux format: "? (192.168.1.1) at aa:bb:cc:dd:ee:ff on en0 ifscope [ethernet]"
			// or: "192.168.1.1 ether aa:bb:cc:dd:ee:ff C en0"
			fields := strings.Fields(line)
			for i, field := range fields {
				// Look for MAC address pattern (xx:xx:xx:xx:xx:xx)
				if strings.Count(field, ":") == 5 && len(field) == 17 {
					return field
				}
				// Also check for (incomplete) marker
				if field == "at" && i+1 < len(fields) {
					mac := fields[i+1]
					if strings.Count(mac, ":") == 5 {
						return mac
					}
				}
			}
		case "windows":
			// Windows format: "  192.168.1.1          aa-bb-cc-dd-ee-ff     dynamic"
			fields := strings.Fields(line)
			for _, field := range fields {
				if strings.Count(field, "-") == 5 && len(field) == 17 {
					// Convert Windows format (aa-bb-cc-dd-ee-ff) to standard (aa:bb:cc:dd:ee:ff)
					return strings.ReplaceAll(field, "-", ":")
				}
			}
		}
	}
	return ""
}
