package probes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/lo"
	"github.com/solo-io/go-utils/contextutils"
)

type ServerOptions struct {
	Port         int
	Path         string
	ResponseCode int
	ResponseBody string
}

func DefaultProbeServerOptions() ServerOptions {
	return ServerOptions{
		Port:         8080,
		Path:         "/healthz",
		ResponseCode: http.StatusOK,
		ResponseBody: "OK",
	}
}

func StartProbeServer(ctx context.Context, o ServerOptions) {
	var server *http.Server
	logger := contextutils.LoggerFrom(ctx)
	// run the healthz server
	go func() {
		mux := new(http.ServeMux)
		mux.HandleFunc(o.Path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(o.ResponseCode)
			w.Write([]byte(o.ResponseBody))
		})
		server = &http.Server{
			Addr:    fmt.Sprintf(":%d", o.Port),
			Handler: mux,
		}
		logger.Infof("probe server starting at %s listening for %s", server.Addr, o.Path)
		err := server.ListenAndServe()
		if err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				logger.Info("probe server closed")
			} else {
				logger.Warnf("probe server closed with unexpected error: %v", err)
			}
		}
	}()

	// Run a separate goroutine to handle the server shutdown when the context is cancelled
	go func() {
		<-ctx.Done()
		if lo.IsNotNil(server) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				logger.Warnf("probe server shutdown returned error: %v", err)
			}
		}
	}()
}
