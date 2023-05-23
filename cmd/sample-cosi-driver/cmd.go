// Copyright 2023 The Kubernetes Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"

	"sigs.k8s.io/container-object-storage-interface-provisioner-sidecar/pkg/provisioner"
	"sigs.k8s.io/cosi-driver-sample/pkg/driver"
	"sigs.k8s.io/cosi-driver-sample/pkg/s3fake"
)

const provisionerName = "sample-driver.objectstorage.k8s.io"

var (
	driverAddress = "unix:///var/lib/cosi/cosi.sock"
	s3URL         = "http://s3.sample-cosi-driver.svc.cluster.local:80"
)

var cmd = &cobra.Command{
	Use:           "sample-cosi-driver",
	Short:         "K8s COSI driver reference implementation",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), args)
	},
	DisableFlagsInUseLine: true,
}

func init() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	flag.Set("alsologtostderr", "true")
	kflags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(kflags)

	persistentFlags := cmd.PersistentFlags()
	persistentFlags.AddGoFlagSet(kflags)

	stringFlag := persistentFlags.StringVarP

	stringFlag(&driverAddress,
		"driver-addr",
		"d",
		driverAddress,
		"path to unix domain socket where driver should listen")

	stringFlag(&s3URL,
		"s3-url",
		"s",
		s3URL,
		"URL where S3 server is listening")

	viper.BindPFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if viper.IsSet(f.Name) && viper.GetString(f.Name) != "" {
			cmd.PersistentFlags().Set(f.Name, viper.GetString(f.Name))
		}
	})
}

// run is the main entrypoint for the driver.
func run(ctx context.Context, args []string) error {
	ctx, cancel := setupSignalHandler(ctx)
	defer cancel()

	s3, err := setupS3(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup S3: %w", err)
	}

	server, err := setupDriver(ctx, s3URL, s3)
	if err != nil {
		return fmt.Errorf("failed to setup driver: %w", err)
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	// Run the driver server as separate goroutine
	go func(ctx context.Context) {
		defer wg.Done()
		if err := server.Run(ctx); err != nil {
			klog.Fatalf("driver server failed: %v", err)
		}
	}(ctx)

	// Run the S3 server as separate goroutine
	go func(ctx context.Context) {
		defer wg.Done()
		if err := s3.Run(ctx); err != nil {
			klog.Fatalf("s3 server failed: %v", err)
		}
	}(ctx)

	wg.Wait()

	return nil
}

// setupSignalHandler creates a context that is canceled when SIGTERM or SIGINT is received.
func setupSignalHandler(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
}

// setupS3 creates s3 fake service. It is only for demo purpose.
// In real world, S3 server should be a separate component, e.g. MinIO, AWS S3, etc.
func setupS3(ctx context.Context) (*s3fake.S3Fake, error) {
	s3 := &s3fake.S3Fake{
		Backend: s3mem.New(),
		Address: s3URL,
	}

	return s3, nil
}

// setupDriver creates COSI provisioner server.
func setupDriver(ctx context.Context,
	s3url string, s3 *s3fake.S3Fake) (*provisioner.COSIProvisionerServer, error) {
	bucketProvisioner := &driver.ProvisionerServer{
		Provisioner: provisionerName,
		S3URL:       s3url,
		S3:          s3,
	}

	identityServer := &driver.IdentityServer{
		Provisioner: provisionerName,
	}

	server, err := provisioner.NewDefaultCOSIProvisionerServer(driverAddress,
		identityServer,
		bucketProvisioner)
	if err != nil {
		return nil, fmt.Errorf("failed to create COSI provisioner server: %w", err)
	}

	return server, nil
}
