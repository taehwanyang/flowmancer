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
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 16384);
    __type(key, __u64);
    __type(value, struct connect_entry_state);
} inflight SEC(".maps");

static __always_inline __u32 get_netns_ino(struct sock *sk)
{
    struct net *net = BPF_CORE_READ(sk, __sk_common.skc_net.net);
    return BPF_CORE_READ(net, ns.inum);
}

static __always_inline void fill_common_state(struct connect_entry_state *s, struct sock *sk)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    __u64 uid_gid = bpf_get_current_uid_gid();

    s->ts_ns = bpf_ktime_get_ns();
    s->pid = (__u32)pid_tgid;
    s->tgid = (__u32)(pid_tgid >> 32);
    s->uid = (__u32)uid_gid;
    s->netns_ino = get_netns_ino(sk);
    bpf_get_current_comm(&s->comm, sizeof(s->comm));
}

static __always_inline int enter_ipv4(struct sock *sk, struct sockaddr *uaddr)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct connect_entry_state s = {};
    struct sockaddr_in sa = {};

    fill_common_state(&s, sk);
    s.family = FLOWMANCER_AF_INET;

    if (uaddr) {
        bpf_probe_read_kernel(&sa, sizeof(sa), uaddr);
        __builtin_memcpy(&s.daddr_v6[0], &sa.sin_addr.s_addr, sizeof(sa.sin_addr.s_addr));
        s.dport = bpf_ntohs(sa.sin_port);
    }

    bpf_map_update_elem(&inflight, &pid_tgid, &s, BPF_ANY);
    return 0;
}

static __always_inline int enter_ipv6(struct sock *sk, struct sockaddr *uaddr)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct connect_entry_state s = {};
    struct sockaddr_in6 sa6 = {};

    fill_common_state(&s, sk);
    s.family = FLOWMANCER_AF_INET6;

    if (uaddr) {
        bpf_probe_read_kernel(&sa6, sizeof(sa6), uaddr);
        __builtin_memcpy(&s.daddr_v6[0], &sa6.sin6_addr.in6_u.u6_addr8, 16);
        s.dport = bpf_ntohs(sa6.sin6_port);
    }

    bpf_map_update_elem(&inflight, &pid_tgid, &s, BPF_ANY);
    return 0;
}

static __always_inline int submit_from_state(__s32 ret)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct connect_entry_state *s;
    struct tcp_connect_event *e;

    s = bpf_map_lookup_elem(&inflight, &pid_tgid);
    if (!s)
        return 0;

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) {
        bpf_map_delete_elem(&inflight, &pid_tgid);
        return 0;
    }

    __builtin_memset(e, 0, sizeof(*e));

    e->ts_ns = s->ts_ns;
    e->end_ts_ns = bpf_ktime_get_ns();
    e->pid = s->pid;
    e->tgid = s->tgid;
    e->uid = s->uid;
    e->netns_ino = s->netns_ino;
    e->ret = ret;
    e->family = s->family;
    e->dport = s->dport;

    __builtin_memcpy(&e->comm[0], &s->comm[0], sizeof(e->comm));
    if (s->family == FLOWMANCER_AF_INET) {
        __builtin_memcpy(&e->daddr_v6[0], &s->daddr_v6[0], 4);
    } else {
        __builtin_memcpy(&e->daddr_v6[0], &s->daddr_v6[0], 16);
    }

    bpf_ringbuf_submit(e, 0);
    bpf_map_delete_elem(&inflight, &pid_tgid);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(handle_tcp_v4_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len)
{
    return enter_ipv4(sk, uaddr);
}

SEC("kretprobe/tcp_v4_connect")
int BPF_KRETPROBE(handle_tcp_v4_connect_ret, int ret)
{
    return submit_from_state(ret);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(handle_tcp_v6_connect, struct sock *sk, struct sockaddr *uaddr, int addr_len)
{
    return enter_ipv6(sk, uaddr);
}

SEC("kretprobe/tcp_v6_connect")
int BPF_KRETPROBE(handle_tcp_v6_connect_ret, int ret)
{
    return submit_from_state(ret);
}