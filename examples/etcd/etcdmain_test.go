// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcd

import (
	"log/slog"

	zapslog "github.com/tommoulard/zap-slog"
	"go.etcd.io/etcd/pkg/v3/osutil"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
)

func MakeZapLogger() *zap.Logger {
	lg, err := logutil.DefaultZapLoggerConfig.Build(zapslog.WrapCore(slog.Default()))
	if err != nil {
		panic(err)
	}
	return lg
}

// EtcdMain is a simplified version of the main function in the etcdmain
// package modified to take a *embed.Config as argument. The test configures the
// logger using the config.
func EtcdMain(cfg *embed.Config) {
	if err := cfg.Validate(); err != nil {
		panic(err)
	}
	cfg.SetupGlobalLoggers()
	lg := cfg.GetLogger()

	e, err := embed.StartEtcd(cfg)
	if err != nil {
		lg.Fatal("start failed", zap.Error(err))
	}
	osutil.RegisterInterruptHandler(e.Close)
	select {
	case <-e.Server.ReadyNotify(): // wait for e.Server to join the cluster
	case <-e.Server.StopNotify(): // publish aborted from 'ErrStopped'
	}
	stopped := e.Server.StopNotify()
	errc := e.Err()

	osutil.HandleInterrupts(lg)

	select {
	case lerr := <-errc:
		// fatal out on listener errors
		lg.Fatal("listener failed", zap.Error(lerr))
	case <-stopped:
	}

	osutil.Exit(0)
}
