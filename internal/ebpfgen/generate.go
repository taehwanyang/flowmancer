package ebpfgen

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -I../../bpf -I../../bpf/headers" Flow ../../bpf/tcp_connect.c -- -D__TARGET_ARCH_arm64
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -cflags "-O2 -g -Wall -Werror -I../../bpf -I../../bpf/headers" DNS ../../bpf/dns_capture.c -- -D__TARGET_ARCH_arm64
