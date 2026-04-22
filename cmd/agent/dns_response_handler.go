package main

import (
	"github.com/taehwanyang/flowmancer/internal/dns"
)

type DNSRespHandler struct {
	dnsCache *dns.Cache
}

func NewDNSRespHandler(dnsCache *dns.Cache) *DNSRespHandler {
	return &DNSRespHandler{
		dnsCache: dnsCache,
	}
}

func (h *DNSRespHandler) Handle(resp *dns.ParsedResponse) {
	if resp == nil || resp.Domain == "" || len(resp.IPs) == 0 {
		return
	}

	h.dnsCache.Add(resp.Domain, resp.IPs, resp.TTL)
}
