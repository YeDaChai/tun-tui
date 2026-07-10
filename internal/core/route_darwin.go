//go:build darwin

package core

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

const tunGateway4 = "198.18.0.1"

// CleanupTunRoutes 删除 Mihomo TUN 残留的路由条目。
// macOS 上 sing-tun 关闭 utun 时不一定会撤销 auto-route，需要主动清理。
func CleanupTunRoutes() error {
	gateway := netip.MustParseAddr(tunGateway4)

	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return fmt.Errorf("fetch routes: %w", err)
	}
	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return fmt.Errorf("parse routes: %w", err)
	}

	var firstErr error
	for _, raw := range messages {
		rm, ok := raw.(*route.RouteMessage)
		if !ok || rm.Type != unix.RTM_GET {
			continue
		}
		prefix, gw, ok := routeEntry(rm)
		if !ok || gw != gateway {
			continue
		}
		if err := deleteRoute(prefix, gw); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func routeEntry(rm *route.RouteMessage) (netip.Prefix, netip.Addr, bool) {
	if len(rm.Addrs) <= unix.RTAX_DST || rm.Addrs[unix.RTAX_DST] == nil {
		return netip.Prefix{}, netip.Addr{}, false
	}

	var dst netip.Addr
	var bits int

	switch addr := rm.Addrs[unix.RTAX_DST].(type) {
	case *route.Inet4Addr:
		dst = netip.AddrFrom4(addr.IP)
		bits = 32
		if rm.Addrs[unix.RTAX_NETMASK] != nil {
			if mask, ok := rm.Addrs[unix.RTAX_NETMASK].(*route.Inet4Addr); ok {
				ones, _ := net.IPMask(mask.IP[:]).Size()
				bits = ones
			}
		}
	case *route.Inet6Addr:
		dst = netip.AddrFrom16(addr.IP)
		bits = 128
		if rm.Addrs[unix.RTAX_NETMASK] != nil {
			if mask, ok := rm.Addrs[unix.RTAX_NETMASK].(*route.Inet6Addr); ok {
				ones, _ := net.IPMask(mask.IP[:]).Size()
				bits = ones
			}
		}
	default:
		return netip.Prefix{}, netip.Addr{}, false
	}

	var gw netip.Addr
	if rm.Addrs[unix.RTAX_GATEWAY] != nil {
		switch addr := rm.Addrs[unix.RTAX_GATEWAY].(type) {
		case *route.Inet4Addr:
			gw = netip.AddrFrom4(addr.IP)
		case *route.Inet6Addr:
			gw = netip.AddrFrom16(addr.IP)
		}
	}

	if !gw.IsValid() {
		return netip.Prefix{}, netip.Addr{}, false
	}
	return netip.PrefixFrom(dst, bits), gw, true
}

func deleteRoute(destination netip.Prefix, gateway netip.Addr) error {
	msg := route.RouteMessage{
		Type:    unix.RTM_DELETE,
		Flags:   unix.RTF_UP | unix.RTF_STATIC | unix.RTF_GATEWAY,
		Version: unix.RTM_VERSION,
		Seq:     1,
	}
	if gateway.Is4() {
		msg.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet4Addr{IP: destination.Addr().As4()},
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: netip.MustParseAddr(net.IP(net.CIDRMask(destination.Bits(), 32)).String()).As4()},
			syscall.RTAX_GATEWAY: &route.Inet4Addr{IP: gateway.As4()},
		}
	} else {
		msg.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet6Addr{IP: destination.Addr().As16()},
			syscall.RTAX_NETMASK: &route.Inet6Addr{IP: netip.MustParseAddr(net.IP(net.CIDRMask(destination.Bits(), 128)).String()).As16()},
			syscall.RTAX_GATEWAY: &route.Inet6Addr{IP: gateway.As16()},
		}
	}
	request, err := msg.Marshal()
	if err != nil {
		return err
	}
	return routeSocketWrite(request)
}

func routeSocketWrite(request []byte) error {
	fd, err := unix.Socket(unix.AF_ROUTE, unix.SOCK_RAW, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	_, err = unix.Write(fd, request)
	return err
}
