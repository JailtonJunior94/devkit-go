package sqlshared_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	internalpool "github.com/JailtonJunior94/devkit-go/pkg/database/internal/pool"
	"github.com/JailtonJunior94/devkit-go/pkg/database/internal/sqlshared"
)

var (
	pingDelayNs      atomic.Int64
	stubRegisterOnce sync.Once
)

type stubDriver struct{}

type stubConn struct{}

type stubTx struct{}

func registerStubDriver() {
	stubRegisterOnce.Do(func() {
		sql.Register("sqlshared-stub", stubDriver{})
	})
}

func (stubDriver) Open(_ string) (driver.Conn, error) { return stubConn{}, nil }

func (stubConn) Ping(ctx context.Context) error {
	delay := time.Duration(pingDelayNs.Load())
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (stubConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("stub: prepare not implemented")
}

func (stubConn) Close() error              { return nil }
func (stubConn) Begin() (driver.Tx, error) { return stubTx{}, nil }

func (stubTx) Commit() error   { return nil }
func (stubTx) Rollback() error { return nil }

func TestOpen_RespectsPingTimeout(t *testing.T) {
	registerStubDriver()
	pingDelayNs.Store(int64(2 * time.Second))
	t.Cleanup(func() { pingDelayNs.Store(0) })

	start := time.Now()
	_, err := sqlshared.Open(sqlshared.OpenParams{
		Driver:      database.Driver("stub"),
		DriverName:  "sqlshared-stub",
		DSN:         "stub",
		PingTimeout: 100 * time.Millisecond,
		Info:        internalpool.ConnInfo{},
	})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, time.Second, "ping deve abortar dentro do PingTimeout configurado")
}

func TestOpen_DefaultPingTimeoutAppliedWhenZero(t *testing.T) {
	registerStubDriver()
	pingDelayNs.Store(0)

	a, err := sqlshared.Open(sqlshared.OpenParams{
		Driver:     database.Driver("stub"),
		DriverName: "sqlshared-stub",
		DSN:        "stub",
		Info:       internalpool.ConnInfo{},
	})
	require.NoError(t, err)
	require.NoError(t, a.Close(context.Background()))
}

func TestBeginTx_PoolExhaustionAbortsOnCtxDeadline(t *testing.T) {
	registerStubDriver()
	pingDelayNs.Store(0)

	a, err := sqlshared.Open(sqlshared.OpenParams{
		Driver:     database.Driver("stub"),
		DriverName: "sqlshared-stub",
		DSN:        "stub",
		Settings:   sqlshared.ConnSettings{MaxOpenConns: 1},
		Info:       internalpool.ConnInfo{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close(context.Background()) })

	tx1, err := a.BeginTx(context.Background(), database.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx1.Rollback(context.Background()) })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = a.BeginTx(ctx, database.TxOptions{})
	elapsed := time.Since(start)

	require.Error(t, err, "segunda BeginTx deve falhar quando pool exhausted e ctx expira")
	require.Less(t, elapsed, time.Second, "BeginTx deve respeitar deadline na fila do pool")
}
