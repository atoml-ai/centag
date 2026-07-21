module centag/dist/personal

go 1.25.0

require (
	centag/apps/wrap v0.0.0
	centag/core v0.0.0
	centag/plugins/backend/anthropic v0.0.0-00010101000000-000000000000
	centag/plugins/backend/azure v0.0.0-00010101000000-000000000000
	centag/plugins/backend/gemini v0.0.0-00010101000000-000000000000
	centag/plugins/backend/ollama v0.0.0-00010101000000-000000000000
	centag/plugins/backend/openai v0.0.0-00010101000000-000000000000
	centag/plugins/database/postgresql v0.0.0
	centag/plugins/database/sqlite v0.0.0
	centag/plugins/protocol/anthropic v0.0.0-00010101000000-000000000000
	centag/plugins/protocol/gemini v0.0.0-00010101000000-000000000000
	centag/plugins/protocol/openai v0.0.0
	centag/plugins/protocol/openairesponses v0.0.0
	centag/plugins/storage/chroma v0.0.0
	centag/plugins/storage/elasticsearch v0.0.0
	centag/plugins/storage/file v0.0.0
	centag/plugins/storage/postgresql v0.0.0
	centag/plugins/storage/redis v0.0.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/sonic v1.9.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.9.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.7.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.4 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.52.0 // indirect
)

replace (
	centag/apps/wrap => ../../apps/wrap
	centag/core => ../../core
	centag/plugins/backend/anthropic => ../../plugins/backend/anthropic
	centag/plugins/backend/azure => ../../plugins/backend/azure
	centag/plugins/backend/gemini => ../../plugins/backend/gemini
	centag/plugins/backend/ollama => ../../plugins/backend/ollama
	centag/plugins/backend/openai => ../../plugins/backend/openai

	centag/plugins/database/postgresql => ../../plugins/database/postgresql
	centag/plugins/database/sqlite => ../../plugins/database/sqlite
	centag/plugins/protocol/anthropic => ../../plugins/protocol/anthropic
	centag/plugins/protocol/gemini => ../../plugins/protocol/gemini
	centag/plugins/protocol/openai => ../../plugins/protocol/openai
	centag/plugins/protocol/openairesponses => ../../plugins/protocol/openairesponses
	centag/plugins/storage/chroma => ../../plugins/storage/chroma
	centag/plugins/storage/elasticsearch => ../../plugins/storage/elasticsearch
	centag/plugins/storage/file => ../../plugins/storage/file
	centag/plugins/storage/postgresql => ../../plugins/storage/postgresql
	centag/plugins/storage/redis => ../../plugins/storage/redis
)
