package main

import (
	"log"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type TCPConnectEventHandler struct {
	resolver    *k8smeta.SrcResolver
	dnsCache    *dns.Cache
	dstResolver *k8smeta.DstResolver
	agg         *aggregator.WorkloadBaselineAggregator
}

func NewTCPConnectEventHandler(
	resolver *k8smeta.SrcResolver,
	dnsCache *dns.Cache,
	dstResolver *k8smeta.DstResolver,
	agg *aggregator.WorkloadBaselineAggregator) *TCPConnectEventHandler {
	return &TCPConnectEventHandler{
		resolver:    resolver,
		dnsCache:    dnsCache,
		dstResolver: dstResolver,
		agg:         agg,
	}
}

func (h *TCPConnectEventHandler) Handle(ev model.TCPConnectEvent) {
	var pod *k8smeta.PodMetadata

	if resolved, ok := h.resolver.ResolveNetns(ev.NetnsIno); ok {
		pod = &resolved
		log.Printf(
			"[resolved] netns=%d -> %s/%s workload=%s/%s",
			ev.NetnsIno,
			resolved.Namespace,
			resolved.PodName,
			resolved.WorkloadKind,
			resolved.WorkloadName,
		)
	} else {
		log.Printf("[resolved] netns=%d -> <unresolved>", ev.NetnsIno)
	}

	dstIP := ev.DstIP()
	if v4 := dstIP.To4(); v4 != nil {
		dstIP = v4
	}

	var domain string
	if d, ok := h.dnsCache.Lookup(dstIP); ok {
		domain = d
		log.Printf("[dns lookup] hit dst=%s domain=%s", dstIP.String(), domain)
	} else {
		log.Printf("[dns lookup] miss dst=%s", dstIP.String())
	}

	var dstK8sName string
	if resolvedDst, ok := h.dstResolver.ResolveDstIP(dstIP); ok {
		dstK8sName = resolvedDst.Name
		log.Printf("[k8s dst] hit dst=%s name=%s", dstIP.String(), dstK8sName)
	} else {
		log.Printf("[k8s dst] miss dst=%s", dstIP.String())
	}

	h.agg.Add(aggregator.ResolvedFlow{
		Event:      ev,
		Pod:        pod,
		Domain:     domain,
		DstK8sName: dstK8sName,
	})
}
