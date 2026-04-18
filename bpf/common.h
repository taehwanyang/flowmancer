#ifndef __FLOWMANCER_COMMON_H__
#define __FLOWMANCER_COMMON_H__

#define FLOWMANCER_COMM_LEN 16
#define FLOWMANCER_AF_INET 2
#define FLOWMANCER_AF_INET6 10

struct connect_entry_state {
    __u64 ts_ns;
    __u32 pid;
    __u32 tgid;
    __u32 uid;
    __u32 netns_ino;

    __u16 family;
    __u16 dport;
    __u16 _pad16;
    __u32 _pad32;

    char comm[FLOWMANCER_COMM_LEN];
    __u8 daddr_v6[16];
};

struct tcp_connect_event {
    __u64 ts_ns;
    __u64 end_ts_ns;

    __u32 pid;
    __u32 tgid;
    __u32 uid;
    __u32 netns_ino;

    __s32 ret;
    __u16 family;
    __u16 dport;
    __u16 sport;
    __u16 _pad;

    char comm[FLOWMANCER_COMM_LEN];

    __u8 saddr_v6[16];
    __u8 daddr_v6[16];
};

#endif