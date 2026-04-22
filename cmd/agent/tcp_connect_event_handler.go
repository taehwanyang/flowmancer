package main

import (
	"log"
	"time"

	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
)

type TCPConnectEventHandler struct {
	srcResolver *k8smeta.SrcResolver
	dnsCache    *dns.Cache
	dstResolver *k8smeta.DstResolver

	builder    *aggregator.BaselineBuilder
	windowAgg  *aggregator.WorkloadWindowAggregator
	detectCh   chan<- aggregator.ClosedWindow
	buildUntil time.Time
}

func NewTCPConnectEventHandler(
	srcResolver *k8smeta.SrcResolver,
	dnsCache *dns.Cache,
	dstResolver *k8smeta.DstResolver,
	builder *aggregator.BaselineBuilder,
	windowAgg *aggregator.WorkloadWindowAggregator,
	detectCh chan<- aggregator.ClosedWindow,
	buildUntil time.Time,
) *TCPConnectEventHandler {
	return &TCPConnectEventHandler{
		srcResolver: srcResolver,
		dnsCache:    dnsCache,
		dstResolver: dstResolver,
		builder:     builder,
		windowAgg:   windowAgg,
		detectCh:    detectCh,
		buildUntil:  buildUntil,
	}
}

func (h *TCPConnectEventHandler) Handle(ev model.TCPConnectEvent) {
	var pod *k8smeta.PodMetadata

	if resolved, ok := h.srcResolver.ResolveNetns(ev.NetnsIno); ok {
		pod = &resolved
		// log.Printf(
		// 	"[Network Namespace -> Pod Resolved] netns=%d -> pod=%s/%s",
		// 	ev.NetnsIno,
		// 	resolved.Namespace,
		// 	resolved.PodName,
		// )
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

	// if domain != "" || dstK8sName != "" {
	// 	log.Printf("[Dest IP -> Domain or K8S] hit dstIP=%s dst domain=%s k8s=%s", dstIP, domain, dstK8sName)
	// }

	resolvedFlow := aggregator.ResolvedFlow{
		Event:      ev,
		Pod:        pod,
		Domain:     domain,
		DstK8sName: dstK8sName,
	}

	now := ev.Time()

	if now.Before(h.buildUntil) {
		h.builder.Add(resolvedFlow)
	}

	h.windowAgg.Add(resolvedFlow)

	closed := h.windowAgg.PopExpired(now)
	for _, cw := range closed {
		select {
		case h.detectCh <- cw:
		default:
			log.Printf(
				"[warn] detect channel full, dropping closed window subject=%s dst=%s:%d window=%s~%s",
				aggregator.SubjectStringFromKey(cw.Key),
				cw.Key.Dst,
				cw.Key.DstPort,
				cw.WindowStart.Format(time.RFC3339),
				cw.WindowEnd.Format(time.RFC3339),
			)
		}
	}
}
