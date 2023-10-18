package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/doomshrine/gocosi"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/hellofresh/health-go/v5"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0" // FIXME: this might need manual update
	"golang.org/x/sync/errgroup"

	"github.com/shanduur/cosi-driver-sample-s3-inmemory/internal/s3fake"
	"github.com/shanduur/cosi-driver-sample-s3-inmemory/servers/identity"
	"github.com/shanduur/cosi-driver-sample-s3-inmemory/servers/provisioner"
)

var (
	ospName    = "cosi.example.com" // FIXME: replace with your own OSP name
	ospVersion = "v0.1.0"           // FIXME: replace with your own OSP version

	exporterKind = gocosi.HTTPExporter

	log logr.Logger

	healthcheck bool
	s3URL       string
)

func main() {
	flag.BoolVar(&healthcheck, "healthcheck", false, "")
	flag.StringVar(&s3URL, "healthcheck", "0.0.0.0:80", "")
	flag.Parse()

	// Setup your logger here.
	// You can use one of multiple available implementation, like:
	//   - https://github.com/kubernetes/klog/tree/main/klogr
	//   - https://github.com/go-logr/logr/tree/master/slogr
	//   - https://github.com/go-logr/stdr
	//   - https://github.com/bombsimon/logrusr
	stdr.SetVerbosity(10)
	log = stdr.New(stdlog.New(os.Stdout, "", stdlog.LstdFlags))

	gocosi.SetLogger(log)

	if err := realMain(context.Background()); err != nil {
		log.Error(err, "critical failure")
		os.Exit(1)
	}
}

func realMain(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if healthcheck {
		return runHealthcheck(ctx)
	}

	return runOSP(ctx)
}

func runHealthcheck(ctx context.Context) error {
	err := gocosi.HealthcheckFunc(ctx, gocosi.HealthcheckAddr)
	if err != nil {
		return fmt.Errorf("healthcheck call failed: %w", err)
	}

	return nil
}

func runOSP(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(ospName),
		semconv.ServiceVersion(ospVersion),
	)

	// If there is any additional confifuration needed for your COSI Driver,
	// put it below this line.
	s3 := s3fake.NewS3Fake(log, s3URL, s3mem.New())

	driver, err := gocosi.New(
		identity.New(ospName, log),
		provisioner.New(log, s3),
		res,
		gocosi.WithHealthcheck(
			health.WithComponent(health.Component{
				Name:    ospName,
				Version: ospVersion,
			}),
		),
		gocosi.WithDefaultGRPCOptions(),
		gocosi.WithDefaultMetricExporter(exporterKind),
		gocosi.WithDefaultTraceExporter(exporterKind),
	)
	if err != nil {
		return fmt.Errorf("failed to create COSI OSP: %w", err)
	}

	eg := new(errgroup.Group)

	// Run the S3 server as separate goroutine
	eg.Go(func() error {
		log.V(3).Info("starting s3 server", "address", s3URL)
		if err := s3.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel()
			log.Error(err, "s3 failure")
			return fmt.Errorf("running S3 server failed: %w", err)
		}

		return nil
	})

	eg.Go(func() error {
		log.V(3).Info("starting driver server")
		if err := driver.Run(ctx); err != nil {
			cancel()
			log.Error(err, "driver failure")
			return fmt.Errorf("failed to run COSI OSP: %w", err)
		}

		return nil
	})

	return eg.Wait()
}
