//go:build ignore
#include "vmlinux.h"
#include "common.h"

#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#ifndef TC_ACT_OK
#define TC_ACT_OK 0
#endif

#ifndef ETH_P_IP
#define ETH_P_IP 0x0800
#endif

#ifndef IPPROTO_UDP
#define IPPROTO_UDP 17
#endif

#ifndef AF_INET
#define AF_INET 2
#endif

char LICENSE[] SEC("license") = "Dual MIT/GPL";

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} dns_events SEC(".maps");

static __always_inline int parse_ipv4_dns(struct __sk_buff *skb)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;

    if (bpf_ntohs(eth->h_proto) != ETH_P_IP)
        return TC_ACT_OK;

    struct iphdr *iph = (void *)(eth + 1);
    if ((void *)(iph + 1) > data_end)
        return TC_ACT_OK;

    if (iph->protocol != IPPROTO_UDP)
        return TC_ACT_OK;

    __u32 ihl_len = iph->ihl * 4;
    if (ihl_len < sizeof(*iph))
        return TC_ACT_OK;

    if ((void *)iph + ihl_len > data_end)
        return TC_ACT_OK;

    struct udphdr *udp = (void *)iph + ihl_len;
    if ((void *)(udp + 1) > data_end)
        return TC_ACT_OK;

    if (udp->dest != bpf_htons(53) && udp->source != bpf_htons(53))
        return TC_ACT_OK;

    void *dns = (void *)(udp + 1);
    if (dns > data_end)
        return TC_ACT_OK;

    __u32 payload_len = (__u32)((long)data_end - (long)dns);
    if (payload_len > sizeof(((struct dns_event *)0)->payload))
        payload_len = sizeof(((struct dns_event *)0)->payload);

    // DNS header minimum size
    if (payload_len < 12)
        return TC_ACT_OK;

    struct dns_event *e = bpf_ringbuf_reserve(&dns_events, sizeof(*e), 0);
    if (!e)
        return TC_ACT_OK;

    __builtin_memset(e, 0, sizeof(*e));

    e->family = AF_INET;
    e->sport = bpf_ntohs(udp->source);
    e->dport = bpf_ntohs(udp->dest);
    e->payload_len = (__u16)payload_len;

    if (bpf_skb_load_bytes(skb,
                           (int)((long)dns - (long)data),
                           e->payload,
                           payload_len) < 0) {
        bpf_ringbuf_discard(e, 0);
        return TC_ACT_OK;
    }

    bpf_ringbuf_submit(e, 0);
    return TC_ACT_OK;
}

SEC("classifier")
int handle_dns_tc(struct __sk_buff *skb)
{
    return parse_ipv4_dns(skb);
}