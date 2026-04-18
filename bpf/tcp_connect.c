//go:build ignore
#include "vmlinux.h"
#include "common.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>

char LICENSE[] SEC("license") = "Dual MIT/GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24); // 16 MiB
} events SEC(".maps");

static __always_inline __u32 get_netns_ino(struct sock *sk)
{
    struct net *net = BPF_CORE_READ(sk, __sk_common.skc_net.net);
    return BPF_CORE_READ(net, ns.inum);
}

static __always_inline int submit_ipv4(struct sock *sk, struct sockaddr *uaddr)
{
    struct tcp_connect_event *e;
    struct sockaddr_in sa = {};

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    __builtin_memset(e, 0, sizeof(*e));

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 uid_gid = bpf_get_current_uid_gid();

    e->ts_ns = bpf_ktime_get_ns();
    e->pid = (__u32)pid_tgid;
    e->tgid = (__u32)(pid_tgid >> 32);
    e->uid = (__u32)uid_gid;
    e->netns_ino = get_netns_ino(sk);
    e->family = FLOWMANCER_AF_INET;

    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    if (uaddr) {
        bpf_probe_read_kernel(&sa, sizeof(sa), uaddr);
        __builtin_memcpy(&e->daddr_v6[0], &sa.sin_addr.s_addr, sizeof(sa.sin_addr.s_addr));
        e->dport = bpf_ntohs(sa.sin_port);
    }

    e->sport = BPF_CORE_READ(sk, __sk_common.skc_num);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

static __always_inline int submit_ipv6(struct sock *sk, struct sockaddr *uaddr)
{
    struct tcp_connect_event *e;
    struct sockaddr_in6 sa6 = {};

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;

    __builtin_memset(e, 0, sizeof(*e));

    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 uid_gid = bpf_get_current_uid_gid();

    e->ts_ns = bpf_ktime_get_ns();
    e->pid = (__u32)pid_tgid;
    e->tgid = (__u32)(pid_tgid >> 32);
    e->uid = (__u32)uid_gid;
    e->netns_ino = get_netns_ino(sk);
    e->family = FLOWMANCER_AF_INET6;

    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    if (uaddr) {
        bpf_probe_read_kernel(&sa6, sizeof(sa6), uaddr);
        __builtin_memcpy(&e->daddr_v6[0], &sa6.sin6_addr.in6_u.u6_addr8, 16);
        e->dport = bpf_ntohs(sa6.sin6_port);
    }

    e->sport = BPF_CORE_READ(sk, __sk_common.skc_num);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(handle_tcp_v4_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len)
{
    return submit_ipv4(sk, uaddr);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(handle_tcp_v6_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len)
{
    return submit_ipv6(sk, uaddr);
}