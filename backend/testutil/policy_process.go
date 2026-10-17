//go:build realpolicy

package testutil

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	osexec "os/exec"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
)

// This file runs the real policy service for tests that must prove an access
// decision, not assume one. The PDP lives under services/policy/internal and
// cannot be imported from another module, and a cross-module require would
// break every service's Dockerfile (each copies only its own go.mod), so the
// harness starts the service as a process and talks gRPC to it.
//
// The tag exists, rather than a t.Skip, because scripts/go-test-strict.sh counts
// a skip as a failure and an untagged `make test` must not compile any of this.
// Under the tag a missing prerequisite is a Fatal: asking for the tag means
// asking for the real policy.

const (
	// PolicyBinEnv names the prebuilt policy binary (`make test-realpolicy`
	// builds it once, outside the per-package test timeout).
	PolicyBinEnv = "NEXUS_POLICY_BIN"
	// TestDatabaseEnv is the Postgres the policy process loads its graph from.
	TestDatabaseEnv = "TEST_DATABASE_URL"

	// unreachableRedis and unreachableKafka are addresses nothing listens on: the
	// policy process must never touch the dev Redis or Redpanda, whose caches and
	// topics real services share.
	unreachableRedis = "redis://127.0.0.1:1/0"
	unreachableKafka = "127.0.0.1:1"

	policyStartTimeout = 45 * time.Second
)

var (
	policyMu   sync.Mutex
	policyProc *policyProcess
)

type policyProcess struct {
	cmd   *osexec.Cmd
	conn  *grpc.ClientConn
	log   *lockedBuffer
	done  chan struct{}
	read  policypb.PolicyReadServiceClient
	write policypb.PolicyWriteServiceClient
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// policyEnv is the whole environment of the policy process. It is built from
// nothing, not from os.Environ(), so no REDIS_URL, KAFKA_BROKERS or database
// address of the developer's shell can leak in.
func policyEnv(databaseURL, grpcPort, metricsPort string) []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"APP_ENV=test",
		"DATABASE_URL=" + databaseURL,
		"GRPC_PORT=" + grpcPort,
		"METRICS_PORT=" + metricsPort,
		"REDIS_URL=" + unreachableRedis,
		"KAFKA_BROKERS=" + unreachableKafka,
		"INTERNAL_IDENTITY_SECRET=" + TestIdentitySecret,
	}
}

func freePort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

// StartPolicy returns clients of the real policy service, starting it on first
// use. One process serves the whole test binary: call RunMain from TestMain so
// it is stopped at exit.
//
// Fixture rule: build graph nodes and edges only through the returned write
// client (see SeedWorkspaceGraph). CreateWorkspace and other SQL fixtures write
// rows the running process never loads, so a decision would not see them; use SQL
// only for business rows. Every RPC other than CreateNode, CreateAssignment,
// DeleteNode and FindNodeByName refuses a bare service identity, so wrap the
// context with PolicyCaller (or PolicyAdmin) first.
func StartPolicy(t testing.TB) (policypb.PolicyReadServiceClient, policypb.PolicyWriteServiceClient) {
	t.Helper()
	policyMu.Lock()
	defer policyMu.Unlock()
	if policyProc == nil {
		p, err := startPolicyProcess()
		if err != nil {
			t.Fatalf("real policy service: %v", err)
		}
		policyProc = p
	}
	return policyProc.read, policyProc.write
}

// PolicyLog returns what the policy process has logged so far, for assertions
// about what it did and did not connect to.
func PolicyLog() string {
	policyMu.Lock()
	defer policyMu.Unlock()
	if policyProc == nil {
		return ""
	}
	return policyProc.log.String()
}

// RunMain is for TestMain: it runs the tests and then stops the policy process.
//
//	func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }
func RunMain(m *testing.M) int {
	code := m.Run()
	StopPolicy()
	return code
}

// StopPolicy stops the policy process, if one was started.
func StopPolicy() {
	policyMu.Lock()
	p := policyProc
	policyProc = nil
	policyMu.Unlock()
	if p == nil {
		return
	}
	_ = p.conn.Close()
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

func startPolicyProcess() (*policyProcess, error) {
	bin := os.Getenv(PolicyBinEnv)
	if bin == "" {
		return nil, fmt.Errorf("%s is not set: run these tests through `make test-realpolicy`, which builds the policy binary once", PolicyBinEnv)
	}
	if _, err := os.Stat(bin); err != nil {
		return nil, fmt.Errorf("%s=%s: %w", PolicyBinEnv, bin, err)
	}
	dbURL := os.Getenv(TestDatabaseEnv)
	if dbURL == "" {
		return nil, fmt.Errorf("%s is not set: the policy process needs a database to load its graph from", TestDatabaseEnv)
	}
	grpcPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("pick gRPC port: %w", err)
	}
	metricsPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("pick metrics port: %w", err)
	}
	if err := grpcauth.Configure(grpcauth.Keys{Current: TestIdentitySecret}); err != nil {
		return nil, fmt.Errorf("configure identity: %w", err)
	}

	p := &policyProcess{log: &lockedBuffer{}, done: make(chan struct{})}
	p.cmd = osexec.Command(bin)
	p.cmd.Env = policyEnv(dbURL, grpcPort, metricsPort)
	p.cmd.Stdout = p.log
	p.cmd.Stderr = p.log
	dieWithParent(p.cmd)
	if err := p.cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	var waitErr error
	go func() {
		waitErr = p.cmd.Wait()
		close(p.done)
	}()
	fail := func(format string, args ...any) (*policyProcess, error) {
		_ = p.cmd.Process.Kill()
		<-p.done
		return nil, fmt.Errorf(format+"\npolicy log:\n%s", append(args, p.log.String())...)
	}

	addr := "127.0.0.1:" + grpcPort
	conn, err := grpcauth.Dial(addr, "auth", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fail("dial %s: %v", addr, err)
	}
	p.conn = conn

	ctx, cancel := context.WithTimeout(context.Background(), policyStartTimeout)
	defer cancel()
	hc := healthpb.NewHealthClient(conn)
	for {
		select {
		case <-p.done:
			_ = conn.Close()
			return nil, fmt.Errorf("policy process exited during start: %v\npolicy log:\n%s", waitErr, p.log.String())
		case <-ctx.Done():
			_ = conn.Close()
			return fail("policy not serving after %s", policyStartTimeout)
		default:
		}
		callCtx, callCancel := context.WithTimeout(ctx, time.Second)
		resp, err := hc.Check(callCtx, &healthpb.HealthCheckRequest{})
		callCancel()
		if err == nil && resp.GetStatus() == healthpb.HealthCheckResponse_SERVING {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	p.read = policypb.NewPolicyReadServiceClient(conn)
	p.write = policypb.NewPolicyWriteServiceClient(conn)
	return p, nil
}

// PolicyCaller returns ctx carrying a verified end user, which is what the
// policy service needs for every RPC that is not service-only.
func PolicyCaller(ctx context.Context, userNodeID string) context.Context {
	return grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: userNodeID, NGACNodeID: userNodeID})
}

// PolicyAdmin is PolicyCaller for the identity that builds fixtures. The policy
// service does not check the caller against the graph, so it needs no node.
func PolicyAdmin(ctx context.Context) context.Context {
	return PolicyCaller(ctx, "harness-admin")
}
