module centag/plugins/storage/redis

go 1.25.0

require (
	github.com/redis/go-redis/v9 v9.17.2
	go.uber.org/zap v1.26.0
	centag/core v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace centag/core => ../../../core
