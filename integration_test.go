//go:build integration

// This file is excluded from `go test ./...` by the `integration` build tag, so
// CI never needs a Docker daemon. It exists so that testcontainers and its
// docker/moby dependency graph are real requirements in go.mod — which is the
// single largest contributor to cold `go mod download` time in this repo.
//
// Run it deliberately with:
//
//	go test -tags integration ./...
package main

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresRoundTrip(t *testing.T) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("demo"),
		postgres.WithUsername("demo"),
		postgres.WithPassword("demo"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer conn.Close(ctx)

	var got int
	require.NoError(t, conn.QueryRow(ctx, "select 1").Scan(&got))
	require.Equal(t, 1, got)
}
