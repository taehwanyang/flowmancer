package main

import (
	"log"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type TCPConnectEventHandler struct {
	srcResolver *k8smeta.SrcResolver
	dnsCache    *dns.Cache
	dstResolver *k8smeta.DstResolver
	agg         *aggregator.WorkloadBaselineAggregator
}

func NewTCPConnectEventHandler(
	srcResolver *k8smeta.SrcResolver,
	dnsCache *dns.Cache,
	dstResolver *k8smeta.DstResolver,
	agg *aggregator.WorkloadBaselineAggregator) *TCPConnectEventHandler {
	return &TCPConnectEventHandler{
		srcResolver: srcResolver,
		dnsCache:    dnsCache,
		dstResolver: dstResolver,
		agg:         agg,
	}
}

func (h *TCPConnectEventHandler) Handle(ev model.TCPConnectEvent) {
	var pod *k8smeta.PodMetadata

	if resolved, ok := h.srcResolver.ResolveNetns(ev.NetnsIno); ok {
		pod = &resolved
		log.Printf(
			"[Network Namespace -> Pod Resolved] netns=%d -> pod=%s/%s",
			ev.NetnsIno,
			resolved.Namespace,
			resolved.PodName,
		)
	}

	dstIP := ev.DstIP()
	if v4 := dstIP.To4(); v4 != nil {
		dstIP = v4
	}

	var domain string
	if d, ok := h.dnsCache.Lookup(dstIP); ok {
		domain = d
	}

	var dstK8sName string
	if resolvedDst, ok := h.dstResolver.ResolveDstIP(dstIP); ok {
		dstK8sName = resolvedDst.Name
	}

	if domain != "" || dstK8sName != "" {
		log.Printf("[Dest IP -> Domain or K8S] hit dstIP=%s dst domain=%s k8s=%s", dstIP, domain, dstK8sName)
	}

	h.agg.Add(aggregator.ResolvedFlow{
		Event:      ev,
		Pod:        pod,
		Domain:     domain,
		DstK8sName: dstK8sName,
	})
}
