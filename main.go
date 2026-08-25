// Command ci-demo-go is a deliberately small HTTP service.
//
// The application is not the point of this repository — the dependency graph
// is. See README.md.
package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// newRouter builds the HTTP surface: one real handler plus a metrics endpoint.
func newRouter(log *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) {
		_, span := otel.Tracer("ci-demo-go").Start(c.Request.Context(), "healthz")
		defer span.End()

		log.Debug("healthz")
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r
}

func main() {
	log, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	ctx := context.Background()

	// Tracing. The exporter dials lazily, so this does not fail without a
	// collector present.
	exp, err := otlptracegrpc.New(ctx)
	if err != nil {
		log.Fatal("otlp exporter", zap.Error(err))
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
	}()

	// Database. Optional: the demo runs fine without one.
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		pool, err := pgxpool.New(ctx, dsn)
		if err != nil {
			log.Fatal("pgxpool", zap.Error(err))
		}
		defer pool.Close()
	}

	addr := ":8080"
	log.Info("listening", zap.String("addr", addr))
	if err := newRouter(log).Run(addr); err != nil {
		log.Fatal("server", zap.Error(err))
	}
}
