package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/bootstrap/redisconn"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/events"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	_ "ngac-platform/services/policy/internal/metrics" // register metrics
	"ngac-platform/services/policy/internal/ngac"
)

func main() {
	bootstrap.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := bootstrap.Env("REDIS_URL", "redis://localhost:6379/0")
	kafkaBrokers := bootstrap.Env("KAFKA_BROKERS", "localhost:19092")
	port := bootstrap.Env("GRPC_PORT", "50051")

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	graph := ngac.NewGraph()
	store := ngac.NewStore(pool, graph)

	if err := store.InitSchema(ctx); err != nil {
		slog.Error("failed to init schema", "error", err)
		os.Exit(1)
	}
	slog.Info("schema initialized")

	if err := store.LoadGraph(ctx); err != nil {
		slog.Error("failed to load graph", "error", err)
		os.Exit(1)
	}
	slog.Info("graph loaded", "nodes", len(graph.Nodes))

	rdb, err := redisconn.Connect(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, access caching disabled", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	producer, err := events.NewProducer(strings.Split(kafkaBrokers, ","))
	if err != nil {
		slog.Warn("kafka unavailable, event streaming disabled", "error", err)
	}
	if producer != nil {
		defer producer.Close()
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(pgrpc.AuthPolicy())...)

	// Shared targeted cache invalidator
	cacheInvalidator := ngac.NewCacheInvalidator(rdb, store.GetGraph)

	// CQRS: Read + Write services
	cte := ngac.NewCTEEvaluator(pool)
	operationStore := ngac.NewOperationStore(pool)
	prohibitionStore := ngac.NewProhibitionStore(pool, store.GetGraph())
	strictOps := os.Getenv("STRICT_OPERATIONS") == "true"
	if strictOps {
		slog.Info("strict operations mode enabled — unregistered operations will be rejected")
	}

	// Assemble read-path components: Cache (PIP) + Engine (PDP) → Evaluator
	decisionCache := ngac.NewLayeredCache(rdb)
	decisionEngine := ngac.NewDecisionEngine(store.GetGraph(), cte)

	// Shard-aware graph resolution: per-workspace subgraph loading + LRU eviction
	shardMgr := ngac.NewShardManager(pool, ngac.ShardManagerConfig{MaxShards: 1000})
	decisionEngine.(interface{ SetShardManager(ngac.ShardManager) }).SetShardManager(shardMgr)
	slog.Info("shard manager wired", "max_shards", 1000)

	evaluator := ngac.NewAccessEvaluator(decisionCache, decisionEngine)

	// Write-path: InvalidationCoordinator hides PIP cache topology from transport
	invalidation := ngac.NewInvalidationCoordinator(cacheInvalidator)

	readServer := pgrpc.NewReadServer(store, rdb, evaluator, operationStore, prohibitionStore)
	pb.RegisterPolicyReadServiceServer(srv, readServer)

	writeServer := pgrpc.NewWriteServer(store, producer, invalidation, operationStore, prohibitionStore, strictOps)
	writeServer.SetShardManager(shardMgr)
	rtProducer := realtime.Connect("policy")
	defer rtProducer.Close()
	writeServer.SetRealtime(rtProducer)
	pb.RegisterPolicyWriteServiceServer(srv, writeServer)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Graceful shutdown
	go bootstrap.Shutdown{GRPC: srv, Health: healthSrv, Cancel: cancel}.Wait()

	// Prometheus metrics HTTP server
	metricsPort := bootstrap.Env("METRICS_PORT", "9090")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		metricsSrv := &http.Server{Addr: ":" + metricsPort, Handler: mux}
		slog.Info("prometheus metrics serving", "port", metricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("metrics server error", "error", err)
		}
	}()

	slog.Info("policy service listening", "port", port)
	if err := srv.Serve(lis); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}
