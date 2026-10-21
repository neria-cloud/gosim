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
	"context"
	"errors"
	"log/slog"

	zapslog "github.com/tommoulard/zap-slog"
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

// RunEmbedded owns one server lifecycle. The caller supplies a buffered
// startup channel and cancels ctx to request a graceful close.
func RunEmbedded(ctx context.Context, cfg *embed.Config, ready chan<- error) error {
	if cfg == nil {
		err := errors.New("nil embedded etcd config")
		ready <- err
		return err
	}
	if err := cfg.Validate(); err != nil {
		ready <- err
		return err
	}
	if err := ctx.Err(); err != nil {
		ready <- err
		return err
	}
	cfg.SetupGlobalLoggers()
	e, err := embed.StartEtcd(cfg)
	if err != nil {
		ready <- err
		return err
	}
	defer e.Close()

	select {
	case <-e.Server.ReadyNotify():
		ready <- nil
	case <-ctx.Done():
		ready <- ctx.Err()
		return ctx.Err()
	case <-e.Server.StopNotify():
		err := errors.New("embedded etcd stopped before readiness")
		ready <- err
		return err
	case err, ok := <-e.Err():
		if !ok || err == nil {
			err = errors.New("embedded etcd error channel closed before readiness")
		}
		ready <- err
		return err
	}

	select {
	case <-ctx.Done():
		return nil
	case <-e.Server.StopNotify():
		return errors.New("embedded etcd stopped without cancellation")
	case err, ok := <-e.Err():
		if !ok || err == nil {
			return errors.New("embedded etcd error channel closed unexpectedly")
		}
		return err
	}
}
