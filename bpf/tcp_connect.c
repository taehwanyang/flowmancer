//go:build ignore
#include "vmlinux.h"
#include "common.h"

#include <bpf/bpf_helpers.h>
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

static __always_inline int submit_ipv4(struct sock *sk)
{
    struct tcp_connect_event *e;

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

    __u32 saddr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    __u32 daddr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    __u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    __be16 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);

    __builtin_memcpy(&e->saddr_v6[0], &saddr, sizeof(saddr));
    __builtin_memcpy(&e->daddr_v6[0], &daddr, sizeof(daddr));
    e->sport = sport;
    e->dport = bpf_ntohs(dport);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

static __always_inline int submit_ipv6(struct sock *sk)
{
    struct tcp_connect_event *e;

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

    struct in6_addr saddr = BPF_CORE_READ(sk, __sk_common.skc_v6_rcv_saddr);
    struct in6_addr daddr = BPF_CORE_READ(sk, __sk_common.skc_v6_daddr);
    __u16 sport = BPF_CORE_READ(sk, __sk_common.skc_num);
    __be16 dport = BPF_CORE_READ(sk, __sk_common.skc_dport);

    __builtin_memcpy(&e->saddr_v6[0], &saddr.in6_u.u6_addr8, 16);
    __builtin_memcpy(&e->daddr_v6[0], &daddr.in6_u.u6_addr8, 16);
    e->sport = sport;
    e->dport = bpf_ntohs(dport);

    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(handle_tcp_v4_connect, struct sock *sk)
{
    return submit_ipv4(sk);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(handle_tcp_v6_connect, struct sock *sk)
{
    return submit_ipv6(sk);
}