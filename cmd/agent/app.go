package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/taehwanyang/flowmancer/config"
	"github.com/taehwanyang/flowmancer/internal/aggregator"
	"github.com/taehwanyang/flowmancer/internal/anomaly"
	"github.com/taehwanyang/flowmancer/internal/dns"
	"github.com/taehwanyang/flowmancer/internal/k8smeta"
	"github.com/taehwanyang/flowmancer/internal/model"
	"github.com/taehwanyang/flowmancer/internal/netutil"
	"github.com/taehwanyang/flowmancer/internal/tcp"
)

type App struct {
	cfg config.Config

	srcResolver *k8smeta.SrcResolver
	dstResolver *k8smeta.DstResolver

	dnsCollector *dns.DNSRespCollector
	tcpCollector *tcp.TCPConnectCollector

	builder        *aggregator.BaselineBuilder
	windowAgg      *aggregator.WorkloadWindowAggregator
	snapshotHolder *aggregator.BaselineSnapshotHolder
	detectCh       chan aggregator.ClosedWindow

	detector *anomaly.Detector
}

func NewApp(cfg config.Config) (*App, error) {
	log.Printf(
		"[config] buildDuration=%s windowSize=%s maxWindowSamples=%d detector.enabled=%v",
		cfg.Server.BuildDuration.Duration,
		cfg.Server.WindowSize.Duration,
		cfg.Server.MaxWindowSamples,
		cfg.Detector.Enabled,
	)

	procRoot := os.Getenv("HOST_PROC")
	if procRoot == "" {
		procRoot = "/proc"
	}
	log.Printf("[config] HOST_PROC=%s", procRoot)
	k8smeta.SetProcRoot(procRoot)

	nodeName := os.Getenv("MY_NODE_NAME")

	clientset, err := k8smeta.NewKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("new kubernetes client: %w", err)
	}

	srcResolver := k8smeta.NewSrcResolver(clientset, nodeName)
	dstResolver := k8smeta.NewDstResolver(clientset)

	return &App{
		cfg:            cfg,
		srcResolver:    srcResolver,
		dstResolver:    dstResolver,
		builder:        aggregator.NewBaselineBuilder(),
		windowAgg:      aggregator.NewWorkloadWindowAggregator(cfg.Server.WindowSize.Duration),
		snapshotHolder: aggregator.NewBaselineSnapshotHolder(),
		detectCh:       make(chan aggregator.ClosedWindow, 1024),
		detector:       newDetectorFromConfig(cfg),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	buildUntil := time.Now().Add(a.cfg.Server.BuildDuration.Duration)

	if err := a.srcResolver.Start(ctx); err != nil {
		return fmt.Errorf("start src resolver: %w", err)
	}

	if err := a.dstResolver.Start(ctx); err != nil {
		return fmt.Errorf("start dst resolver: %w", err)
	}

	if err := a.startCollectors(ctx, buildUntil); err != nil {
		return err
	}

	defer a.Close()

	scheduleBaselineDump(ctx, a.builder, a.cfg.Server.BuildDuration.Duration)

	go a.exportBaselineSnapshotOnce(ctx, buildUntil)
	go runDetectorWorker(ctx, a.detectCh, a.snapshotHolder, a.detector)

	<-ctx.Done()
	return nil
}

func (a *App) startCollectors(ctx context.Context, buildUntil time.Time) error {
	dnsCache := dns.NewCache()

	clockConv, err := model.NewMonotonicClockConverter()
	if err != nil {
		return fmt.Errorf("new monotonic clock converter: %w", err)
	}

	dnsRespHandler := NewDNSRespHandler(dnsCache)
	a.dnsCollector = dns.NewDNSRespCollector(dnsRespHandler.Handle)

	tcpConnectEventHandler := NewTCPConnectEventHandler(
		a.srcResolver,
		dnsCache,
		a.dstResolver,
		a.builder,
		a.windowAgg,
		a.detectCh,
		buildUntil,
		clockConv,
		a.cfg.Server.MaxWindowSamples,
	)

	a.tcpCollector = tcp.NewTCPConnectCollector(
		tcpConnectEventHandler.Handle,
		func(err error) {
			log.Printf("tcp connect collector error: %v", err)
		},
	)

	iface, err := netutil.DetectBridgeInterface()
	if err != nil {
		return fmt.Errorf("find bridge interface: %w", err)
	}

	if err := a.dnsCollector.Start(ctx, iface); err != nil {
		return fmt.Errorf("start dns collector: %w", err)
	}

	if err := a.tcpCollector.Start(ctx); err != nil {
		return fmt.Errorf("start tcp collector: %w", err)
	}

	return nil
}

func (a *App) exportBaselineSnapshotOnce(ctx context.Context, buildUntil time.Time) {
	timer := time.NewTimer(time.Until(buildUntil))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		snapshot := a.builder.ExportSnapshot()
		a.snapshotHolder.Replace(snapshot)
		a.windowAgg.Reset()

		log.Printf("[baseline] snapshot exported")
	}
}

func (a *App) Close() {
	if a.dnsCollector != nil {
		if err := a.dnsCollector.Close(); err != nil {
			log.Printf("close dns collector: %v", err)
		}
	}

	if a.tcpCollector != nil {
		if err := a.tcpCollector.Close(); err != nil {
			log.Printf("close tcp collector: %v", err)
		}
	}

	close(a.detectCh)
}

func newDetectorFromConfig(cfg config.Config) *anomaly.Detector {
	detector := anomaly.NewDetector()

	detector.Enabled = cfg.Detector.Enabled
	detector.NewDestinationEnabled = cfg.Detector.NewDestination.Enabled
	detector.RareDestinationEnabled = cfg.Detector.RareDestination.Enabled
	detector.VolumeAnomalyEnabled = cfg.Detector.VolumeAnomaly.Enabled

	detector.RareConfig = cfg.Detector.RareDestination.ToAnomalyConfig()
	detector.VolumeConfig = cfg.Detector.VolumeAnomaly.ToAnomalyConfig()

	return detector
}
