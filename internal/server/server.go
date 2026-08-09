// SPDX-License-Identifier: Apache-2.0

// Package server exposes the complete Gauge Reporter gRPC service.
package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/collector"
	"google.golang.org/grpc"
)

const maximumMessageBytes = 1024 * 1024 * 1024

// Serve binds loopback on an ephemeral port, prints Gauge's exact handshake,
// and blocks until Kill or context cancellation.
func Serve(ctx context.Context, engine *collector.Engine, logger collector.Logger) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen for Gauge reporter: %w", err)
	}
	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(maximumMessageBytes), grpc.MaxSendMsgSize(maximumMessageBytes))
	handler := &Handler{engine: engine, server: grpcServer}
	gm.RegisterReporterServer(grpcServer, handler)
	if _, err := fmt.Printf("Listening on port:%d\n", listener.Addr().(*net.TCPAddr).Port); err != nil {
		_ = listener.Close()
		return fmt.Errorf("write Gauge startup handshake: %w", err)
	}
	if logger != nil {
		logger.Info("Gauge reporter gRPC server started on loopback")
	}
	done := make(chan error, 1)
	go func() { done <- grpcServer.Serve(listener) }()
	select {
	case err := <-done:
		if err == grpc.ErrServerStopped {
			return nil
		}
		return err
	case <-ctx.Done():
		grpcServer.GracefulStop()
		<-done
		return ctx.Err()
	}
}

// Handler implements every RPC in the current Reporter service explicitly.
type Handler struct {
	gm.UnimplementedReporterServer
	engine *collector.Engine
	server *grpc.Server
	stop   sync.Once
}

func (h *Handler) NotifyExecutionStarting(ctx context.Context, value *gm.ExecutionStartingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.ExecutionStarting(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifySpecExecutionStarting(ctx context.Context, value *gm.SpecExecutionStartingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.SpecStarting(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyScenarioExecutionStarting(ctx context.Context, value *gm.ScenarioExecutionStartingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.ScenarioStarting(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyConceptExecutionStarting(ctx context.Context, value *gm.ConceptExecutionStartingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.ConceptStarting(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyConceptExecutionEnding(ctx context.Context, value *gm.ConceptExecutionEndingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.ConceptEnding(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyStepExecutionStarting(ctx context.Context, value *gm.StepExecutionStartingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.StepStarting(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyStepExecutionEnding(ctx context.Context, value *gm.StepExecutionEndingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.StepEnding(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyScenarioExecutionEnding(ctx context.Context, value *gm.ScenarioExecutionEndingRequest) (*gm.Empty, error) {
	if err := h.engine.ScenarioEnding(ctx, value); err != nil {
		return nil, err
	}
	return &gm.Empty{}, nil
}
func (h *Handler) NotifySpecExecutionEnding(ctx context.Context, value *gm.SpecExecutionEndingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.SpecEnding(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifyExecutionEnding(ctx context.Context, value *gm.ExecutionEndingRequest) (*gm.Empty, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h.engine.ExecutionEnding(value)
	return &gm.Empty{}, nil
}
func (h *Handler) NotifySuiteResult(ctx context.Context, value *gm.SuiteExecutionResult) (*gm.Empty, error) {
	if err := h.engine.Finalize(ctx, value); err != nil {
		return nil, err
	}
	return &gm.Empty{}, nil
}
func (h *Handler) Kill(ctx context.Context, _ *gm.KillProcessRequest) (*gm.Empty, error) {
	if err := h.engine.FinalizeInterrupted(ctx); err != nil {
		return nil, err
	}
	h.stop.Do(func() { go func() { time.Sleep(10 * time.Millisecond); h.server.GracefulStop() }() })
	return &gm.Empty{}, nil
}
