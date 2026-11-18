package limiter

import "net/netip"

// ipv6PrefixBits is the IPv6 block treated as one client. Providers hand a
// single household or server a /64 (or larger), so limiting single IPv6
// addresses would let one client rotate through 2^64 fresh keys.
const ipv6PrefixBits = 64

// ClientKey returns the key that limits one client: the address itself for
// IPv4 (including IPv4-mapped IPv6), and the /64 block for IPv6. Values that
// are not IP addresses are returned unchanged.
func ClientKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		return addr.String()
	}
	prefix, err := addr.Prefix(ipv6PrefixBits)
	if err != nil {
		return ip
	}
	return prefix.String()
}
