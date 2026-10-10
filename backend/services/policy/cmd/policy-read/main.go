package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/bootstrap/redisconn"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/events"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	_ "ngac-platform/services/policy/internal/metrics" // register metrics
	"ngac-platform/services/policy/internal/ngac"
)

func main() {
	bootstrap.InitLogger()
	if err := bootstrap.ConfigureInternalIdentity(); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := bootstrap.Env("REDIS_URL", "redis://localhost:6379/0")
	kafkaBrokers := bootstrap.Env("KAFKA_BROKERS", "localhost:19092")
	port := bootstrap.Env("GRPC_PORT", "50061")

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	graph := ngac.NewGraph()
	store := ngac.NewStore(pool, graph)

	// Mutations are consumed from this point on. It is taken before the graph
	// load (with a margin for clock skew between this host and the broker) so
	// that a mutation committed while the load runs is still delivered.
	consumeMutationsSince := time.Now().Add(-graphSyncLookback)

	if err := store.LoadGraph(ctx); err != nil {
		slog.Error("failed to load graph", "error", err)
		os.Exit(1)
	}
	slog.Info("graph loaded", "nodes", len(graph.Nodes))

	rdb, err := redisconn.Connect(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, L1 caching disabled", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	cte := ngac.NewCTEEvaluator(pool)
	operationStore := ngac.NewOperationStore(pool)
	prohibitionStore := ngac.NewProhibitionStore(pool, store.GetGraph())

	// Assemble read-path components: Cache (PIP) + Engine (PDP) → Evaluator
	decisionCache := ngac.NewLayeredCache(rdb)
	decisionEngine := ngac.NewDecisionEngine(store.GetGraph(), cte)

	// Shard-aware graph resolution: per-workspace subgraph loading + LRU eviction
	shardMgr := ngac.NewShardManager(pool, ngac.ShardManagerConfig{MaxShards: 1000})
	decisionEngine.(interface{ SetShardManager(ngac.ShardManager) }).SetShardManager(shardMgr)
	slog.Info("shard manager wired (read)", "max_shards", 1000)

	evaluator := ngac.NewAccessEvaluator(decisionCache, decisionEngine)

	// Follow the writer: the writer mutates its own in-memory graph, not ours.
	// Each ngac.graph.mutated event reloads this replica's graph and runs the
	// same EPP invalidation the writer runs (shards + InvalidationCoordinator).
	invalidation := ngac.NewInvalidationCoordinator(ngac.NewCacheInvalidator(rdb, store.GetGraph))
	refresher := ngac.NewReplicaGraphRefresher(store, shardMgr, invalidation)
	consumer, err := events.NewGraphMutationConsumer(strings.Split(kafkaBrokers, ","), consumeMutationsSince, refresher)
	if err != nil {
		// A replica that cannot follow mutations would serve its startup graph
		// forever. Refuse to start rather than serve stale decisions.
		slog.Error("failed to create graph mutation consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()
	go consumer.Run(ctx)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(pgrpc.AuthPolicy())...)

	readServer := pgrpc.NewReadServer(store, rdb, evaluator, operationStore, prohibitionStore)
	pb.RegisterPolicyReadServiceServer(srv, readServer)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go bootstrap.Shutdown{GRPC: srv, Health: healthSrv, Cancel: cancel}.Wait()

	// Prometheus metrics HTTP server
	metricsPort := bootstrap.Env("METRICS_PORT", "9091")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv := &http.Server{Addr: ":" + metricsPort, Handler: mux}
		slog.Info("prometheus metrics serving", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("metrics server error", "error", err)
		}
	}()

	slog.Info("policy-read service listening", "port", port)
	if err := srv.Serve(lis); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// graphSyncLookback is how far before the startup graph load mutation events
// are re-read from. Re-applying an event the load already reflected is
// harmless; missing one is not.
const graphSyncLookback = 30 * time.Second
