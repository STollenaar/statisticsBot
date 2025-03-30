module github.com/stollenaar/statisticsbot/internal/commands

go 1.24.1

require (
	github.com/bwmarrin/discordgo v0.28.2-0.20241208071600-33ffff21d31a
	github.com/stollenaar/statisticsbot/internal/commands/countcommand v0.0.0-20250320232739-1e4fd2205923
	github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand v0.0.0-20250320232739-1e4fd2205923
	github.com/stollenaar/statisticsbot/internal/commands/maxcommand v0.0.0-20250320232739-1e4fd2205923
	github.com/stollenaar/statisticsbot/internal/commands/moodcommand v0.0.0-20250320232739-1e4fd2205923
	github.com/stollenaar/statisticsbot/internal/commands/plotcommand v0.0.0-00010101000000-000000000000
	github.com/stollenaar/statisticsbot/internal/commands/summarizecommand v0.0.0-20250320232739-1e4fd2205923
	github.com/stollenaar/statisticsbot/internal/util v0.0.0-20250320232739-1e4fd2205923
)

require (
	github.com/apache/arrow-go/v18 v18.2.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.65 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssm v1.58.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.17 // indirect
	github.com/aws/smithy-go v1.22.3 // indirect
	github.com/chromedp/cdproto v0.0.0-20250319231242-a755498943c8 // indirect
	github.com/chromedp/chromedp v0.13.3 // indirect
	github.com/chromedp/sysutil v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-echarts/go-echarts/v2 v2.5.2 // indirect
	github.com/go-echarts/snapshot-chromedp v0.0.5 // indirect
	github.com/go-json-experiment/json v0.0.0-20250223041408-d3c622f1b874 // indirect
	github.com/go-viper/encoding/ini v0.1.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/marcboeker/go-duckdb v1.8.5 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/sagikazarmark/locafero v0.9.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/spf13/viper v1.20.1 // indirect
	github.com/stollenaar/aws-rotating-credentials-provider/credentials v0.0.0-20250330204128-299effe6093c // indirect
	github.com/stollenaar/statisticsbot/internal/database v0.0.0-20250320232739-1e4fd2205923 // indirect
	github.com/stollenaar/statisticsbot/internal/util/charts v0.0.0-00010101000000-000000000000 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/mod v0.24.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/tools v0.31.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/stollenaar/statisticsbot/internal/commands/countcommand => ./countcommand
	github.com/stollenaar/statisticsbot/internal/commands/lastmessagecommand => ./lastmessagecommand
	github.com/stollenaar/statisticsbot/internal/commands/maxcommand => ./maxcommand
	github.com/stollenaar/statisticsbot/internal/commands/moodcommand => ./moodcommand
	github.com/stollenaar/statisticsbot/internal/commands/plotcommand => ./plotcommand
	github.com/stollenaar/statisticsbot/internal/commands/summarizecommand => ./summarizecommand
	github.com/stollenaar/statisticsbot/internal/database => ../database
	github.com/stollenaar/statisticsbot/internal/util => ../util
	github.com/stollenaar/statisticsbot/internal/util/charts => ../util/charts
)
