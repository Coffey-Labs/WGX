# Performance

ihasvpn is built so the packet path is as short as it can be. This page explains
what it does on its own, what only the host can do, and how to check the
result.

## What decides throughput

In rough order of importance:

1. **Kernel data plane.** WireGuard in the kernel handles encryption in the
   network stack with no copies to user space. ihasvpn creates a native
   `wireguard` link over netlink and configures it over the same channel;
   nothing sits between the NIC and the module. On a host without the module
   ihasvpn falls back to `wireguard-go`, which works everywhere but moves every
   packet through a user-space process and is several times slower. The
   dashboard says which one is in use. Any Linux kernel from 5.6 has the
   module; on older kernels install `wireguard-dkms` on the host.
2. **MTU and MSS.** A tunnel packet has 60 bytes of overhead on IPv4 (80 on
   IPv6). The default MTU of 1420 fits a 1500-byte underlay. If the path to
   the server is smaller than that (PPPoE, a second tunnel, some mobile
   networks) packets fragment or vanish and downloads crawl. ihasvpn clamps the
   TCP MSS of forwarded connections to the route MTU, which removes the
   "connected but pages hang" failure outright; lower the MTU in Settings if
   UDP-heavy traffic still struggles.
3. **Socket buffers.** Bursts on a fast link fill the default UDP receive
   buffer before WireGuard drains it, and the kernel drops the excess. These
   are host-wide sysctls, so see below.
4. **Port mapping.** In the default bridged mode Docker DNATs the UDP port,
   which costs a conntrack lookup per packet. `docker-compose.host.yml` puts
   the socket on the host network and skips it.
5. **The CPU.** WireGuard is ChaCha20-Poly1305 and scales across cores per
   peer; a single flow is bounded by one core. Machines with AVX2 or ARMv8
   crypto extensions do markedly better.

## What ihasvpn sets by itself

At startup ihasvpn writes these through `/proc/sys` and reports the outcome on
the dashboard under **Show kernel tuning**:

| sysctl | value | why |
| --- | --- | --- |
| `net.ipv4.ip_forward` | 1 | required; peers go nowhere without it |
| `net.ipv6.conf.all.forwarding` | 1 | required when `IHASVPN_SUBNET6` is set |
| `net.ipv4.conf.all.rp_filter`, `...default.rp_filter` | 2 | strict reverse-path filtering drops legitimate tunnel replies |
| `net.core.rmem_max`, `net.core.wmem_max` | 26214400 | room for bursts on the UDP socket |
| `net.core.rmem_default`, `net.core.wmem_default` | 1048576 | default socket buffers |
| `net.core.netdev_max_backlog` | 16384 | deeper per-CPU input queue |
| `net.ipv4.udp_rmem_min`, `net.ipv4.udp_wmem_min` | 16384 | UDP buffers under memory pressure |

The first three groups are network-namespaced and work inside the container
when the compose file passes them under `sysctls:` (forwarding) or the
container has NET_ADMIN (rp_filter). The `net.core.*` and `udp_*` ones are
**global**: the kernel refuses them from inside a container, ihasvpn logs a
warning, and they show as "not set" on the dashboard. That is expected; set
them on the host.

## Host settings

Drop this into `/etc/sysctl.d/99-ihasvpn.conf` on the Docker host and run
`sysctl --system`:

```
# Socket buffers: let a 1-10 GbE burst queue rather than drop.
net.core.rmem_max = 26214400
net.core.wmem_max = 26214400
net.core.rmem_default = 1048576
net.core.wmem_default = 1048576
net.core.netdev_max_backlog = 16384
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384

# Fair queueing and BBR help the *host's own* TCP flows; forwarded peer
# traffic keeps the peers' congestion control. Harmless, often helpful.
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr

# Forwarding, in case you run the host-network compose file and want to own
# it yourself (ihasvpn sets it otherwise).
net.ipv4.ip_forward = 1
```

Two more things on the host that are worth checking on a busy server:

- **UDP GRO/GSO offload** on the physical NIC lets the kernel batch
  WireGuard's UDP segments. It is on by default on modern drivers; `ethtool
  -k eth0 | grep -i udp` shows the state.
- **IRQ affinity / multi-queue.** WireGuard spreads decryption across all
  CPUs, but the NIC's receive queues need to be spread too. `irqbalance` on
  most distributions does it; on single-queue virtual NICs there is nothing
  to gain.

## Host networking

`docker-compose.host.yml` runs ihasvpn with `network_mode: host`. It removes the
Docker port mapping from the path and lets ihasvpn set the host's own
forwarding sysctls. The costs are listed at the top of that file; in short,
`wg0` and the `ihasvpn` nftables table become visible on the host, and the admin
UI is bound to localhost so it is not exposed by accident.

## Measuring

Run `iperf3 -s` on a machine behind the server (or on the server itself)
and `iperf3 -c <tunnel address> -P 4` from a peer, then compare it with the
same test outside the tunnel. On a wired gigabit link a modern x86 host with
the kernel module should get within a few percent of line rate; with
`wireguard-go` expect a few hundred Mbit/s and a busy core.

The dashboard's live rates come from the interface counters every two
seconds, so they show what the tunnel actually carries during the test.
