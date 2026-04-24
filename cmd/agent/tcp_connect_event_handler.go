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

	clockConv        *model.MonotonicClockConverter
	maxWindowSamples int
}

func NewTCPConnectEventHandler(
	srcResolver *k8smeta.SrcResolver,
	dnsCache *dns.Cache,
	dstResolver *k8smeta.DstResolver,
	builder *aggregator.BaselineBuilder,
	windowAgg *aggregator.WorkloadWindowAggregator,
	detectCh chan<- aggregator.ClosedWindow,
	buildUntil time.Time,
	clockConv *model.MonotonicClockConverter,
	maxWindowSamples int,
) *TCPConnectEventHandler {
	return &TCPConnectEventHandler{
		srcResolver:      srcResolver,
		dnsCache:         dnsCache,
		dstResolver:      dstResolver,
		builder:          builder,
		windowAgg:        windowAgg,
		detectCh:         detectCh,
		buildUntil:       buildUntil,
		clockConv:        clockConv,
		maxWindowSamples: maxWindowSamples,
	}
}

func (h *TCPConnectEventHandler) Handle(ev model.TCPConnectEvent) {
	var pod *k8smeta.PodMetadata

	if resolved, ok := h.srcResolver.ResolveNetns(ev.NetnsIno); ok {
		pod = &resolved

		// log.Printf(
		// 	"[resolve hit] netns=%d -> %s/%s/%s pod=%s node=%s",
		// 	ev.NetnsIno,
		// 	resolved.Namespace,
		// 	resolved.WorkloadKind,
		// 	resolved.WorkloadName,
		// 	resolved.PodName,
		// 	resolved.NodeName,
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

	resolvedFlow := aggregator.ResolvedFlow{
		Event:      ev,
		Pod:        pod,
		Domain:     domain,
		DstK8sName: dstK8sName,
		ObservedAt: h.clockConv.ToTime(ev.TsNS),
	}

	now := h.clockConv.ToTime(ev.TsNS)

	h.windowAgg.Add(resolvedFlow)

	closed := h.windowAgg.PopExpired(now)

	if now.Before(h.buildUntil) {
		h.handleBaselineLearning(resolvedFlow, closed)
		return
	}

	for _, cw := range closed {
		if !cw.Key.IsResolvedSourceKey() {
			continue
		}

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

func (h *TCPConnectEventHandler) handleBaselineLearning(
	resolvedFlow aggregator.ResolvedFlow,
	closed []aggregator.ClosedWindow,
) {
	if resolvedFlow.Pod != nil {
		h.builder.Add(resolvedFlow)
	}

	for _, cw := range closed {
		if !cw.Key.IsResolvedSourceKey() {
			continue
		}
		h.builder.AppendWindowSample(cw.Key, cw.Count, h.maxWindowSamples)
	}
}
