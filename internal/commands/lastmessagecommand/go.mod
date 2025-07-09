module github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand

go 1.24.1

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/stollenaar/statisticsbot/internal/database v0.0.0-20250709120314-fe0c6fbae883
	github.com/stollenaar/statisticsbot/internal/util v0.0.0-20250709120314-fe0c6fbae883
)

require (
	github.com/apache/arrow-go/v18 v18.2.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.5 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.17 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.70 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.32 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.36 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssm v1.60.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.34.0 // indirect
	github.com/aws/smithy-go v1.22.4 // indirect
	github.com/duckdb/duckdb-go-bindings v0.1.17 // indirect
	github.com/duckdb/duckdb-go-bindings/darwin-amd64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/darwin-arm64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/linux-amd64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/linux-arm64 v0.1.12 // indirect
	github.com/duckdb/duckdb-go-bindings/windows-amd64 v0.1.12 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/encoding/ini v0.1.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/marcboeker/go-duckdb/arrowmapping v0.0.10 // indirect
	github.com/marcboeker/go-duckdb/mapping v0.0.11 // indirect
	github.com/marcboeker/go-duckdb/v2 v2.3.3 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/sagikazarmark/locafero v0.9.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.8.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/spf13/viper v1.20.1 // indirect
	github.com/stollenaar/aws-rotating-credentials-provider/credentials v0.0.0-20250330204128-299effe6093c // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.32.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/stollenaar/statisticsbot/internal/database => ../../database
	github.com/stollenaar/statisticsbot/internal/util => ../../util
)
