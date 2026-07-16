module centag/plugins/storage/chroma

go 1.25.0

require (
	go.uber.org/zap v1.26.0
	centag/core v0.0.0
)

require (
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)

replace centag/core => ../../../core
