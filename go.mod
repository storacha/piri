module github.com/storacha/piri

go 1.24.4

require (
	github.com/aws/aws-lambda-go v1.47.0
	github.com/aws/aws-sdk-go-v2 v1.34.0
	github.com/aws/aws-sdk-go-v2/config v1.28.3
	github.com/aws/aws-sdk-go-v2/credentials v1.17.44
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.15.15
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression v1.7.50
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.36.5
	github.com/aws/aws-sdk-go-v2/service/s3 v1.74.1
	github.com/aws/aws-sdk-go-v2/service/sqs v1.37.0
	github.com/aws/aws-sdk-go-v2/service/ssm v1.55.5
	github.com/awslabs/aws-lambda-go-api-proxy v0.16.2
	github.com/cenkalti/backoff/v5 v5.0.2
	github.com/charmbracelet/bubbles v0.21.0
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/ethereum/go-ethereum v1.14.13
	github.com/filecoin-project/go-commp-utils v0.1.4
	github.com/filecoin-project/go-commp-utils/nonffi v0.0.0-20240802040721-2a04ffc8ffe8
	github.com/filecoin-project/lotus v1.32.0-rc1
	github.com/getsentry/sentry-go v0.31.1
	github.com/glebarez/go-sqlite v1.21.2
	github.com/glebarez/sqlite v1.11.0
	github.com/go-playground/validator/v10 v10.14.0
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/ipfs/go-cid v0.5.0
	github.com/ipfs/go-datastore v0.6.0
	github.com/ipfs/go-ds-leveldb v0.5.0
	github.com/ipfs/go-log/v2 v2.5.1
	github.com/ipld/go-ipld-prime v0.21.1-0.20240917223228-6148356a4c2e
	github.com/ipni/go-libipni v0.6.16
	github.com/labstack/echo/v4 v4.13.3
	github.com/labstack/gommon v0.4.2
	github.com/libp2p/go-libp2p v0.39.1
	github.com/multiformats/go-multiaddr v0.14.0
	github.com/multiformats/go-multibase v0.2.0
	github.com/multiformats/go-multihash v0.2.3
	github.com/ncruces/go-sqlite3 v0.24.1
	github.com/samber/lo v1.39.0
	github.com/snadrus/must v0.0.0-20240605044437-98cedd57f8eb
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.8.1
	github.com/storacha/go-libstoracha v0.0.5
	github.com/storacha/go-ucanto v0.3.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/mock v0.5.0
	golang.org/x/sync v0.12.0
	gorm.io/datatypes v1.2.5
	gorm.io/gorm v1.26.1
	modernc.org/sqlite v1.23.1
	resty.dev/v3 v3.0.0-beta.3
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.17.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/bubbletea v1.3.4 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/x/ansi v0.8.0 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/consensys/bavard v0.1.22 // indirect
	github.com/consensys/gnark-crypto v0.14.0 // indirect
	github.com/crate-crypto/go-ipa v0.0.0-20240724233137-53bbb0ceb27a // indirect
	github.com/crate-crypto/go-kzg-4844 v1.1.0 // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/ethereum/c-kzg-4844 v1.0.0 // indirect
	github.com/ethereum/go-verkle v0.2.2 // indirect
	github.com/filecoin-project/go-amt-ipld/v2 v2.1.0 // indirect
	github.com/filecoin-project/go-amt-ipld/v3 v3.1.0 // indirect
	github.com/filecoin-project/go-amt-ipld/v4 v4.4.0 // indirect
	github.com/filecoin-project/go-bitfield v0.2.4 // indirect
	github.com/filecoin-project/go-clock v0.1.0 // indirect
	github.com/filecoin-project/go-crypto v0.1.0 // indirect
	github.com/filecoin-project/go-f3 v0.7.3 // indirect
	github.com/filecoin-project/go-hamt-ipld v0.1.5 // indirect
	github.com/filecoin-project/go-hamt-ipld/v2 v2.0.0 // indirect
	github.com/filecoin-project/go-hamt-ipld/v3 v3.4.0 // indirect
	github.com/filecoin-project/go-jsonrpc v0.7.0 // indirect
	github.com/filecoin-project/pubsub v1.0.0 // indirect
	github.com/filecoin-project/specs-actors v0.9.15 // indirect
	github.com/filecoin-project/specs-actors/v2 v2.3.6 // indirect
	github.com/filecoin-project/specs-actors/v3 v3.1.2 // indirect
	github.com/filecoin-project/specs-actors/v4 v4.0.2 // indirect
	github.com/filecoin-project/specs-actors/v5 v5.0.6 // indirect
	github.com/filecoin-project/specs-actors/v6 v6.0.2 // indirect
	github.com/filecoin-project/specs-actors/v7 v7.0.1 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.4 // indirect
	github.com/gbrlsnchs/jwt/v3 v3.0.1 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-sql-driver/mysql v1.8.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/hashicorp/golang-lru/arc/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/invopop/jsonschema v0.12.0 // indirect
	github.com/ipfs/boxo v0.21.0 // indirect
	github.com/ipld/go-car/v2 v2.13.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/libp2p/go-flow-metrics v0.2.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/magefile/mage v1.9.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mmcloughlin/addchain v0.4.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/pelletier/go-toml v1.9.3 // indirect
	github.com/petar/GoLLRB v0.0.0-20210522233825-ae3b015fd3e9 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/puzpuzpuz/xsync/v2 v2.4.0 // indirect
	github.com/raulk/clock v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/supranational/blst v0.3.14 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/urfave/cli/v2 v2.27.5 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/whyrusleeping/cbor v0.0.0-20171005072247-63513f603b11 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	gitlab.com/yawning/secp256k1-voi v0.0.0-20230925100816-f2616030848b // indirect
	gitlab.com/yawning/tuplehash v0.0.0-20230713102510-df83abbf9a02 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.50.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.28.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gorm.io/driver/mysql v1.5.6 // indirect
	gorm.io/driver/postgres v1.5.7 // indirect
	gorm.io/driver/sqlite v1.5.7 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	rsc.io/tmplfunc v0.0.3 // indirect
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.8 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.19 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.29 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.29 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.24.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.5.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.32.4 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0 // indirect
	github.com/filecoin-project/go-address v1.2.0
	github.com/filecoin-project/go-commp-utils/v2 v2.1.0
	github.com/filecoin-project/go-data-segment v0.0.1
	github.com/filecoin-project/go-fil-commcid v0.2.0
	github.com/filecoin-project/go-fil-commp-hashhash v0.2.0
	github.com/filecoin-project/go-state-types v0.16.0-rc1
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.5-0.20231225225746-43d5d4cd4e0e // indirect
	github.com/google/uuid v1.6.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-block-format v0.2.0 // indirect
	github.com/ipfs/go-blockservice v0.5.2 // indirect
	github.com/ipfs/go-ipfs-blockstore v1.3.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.1 // indirect
	github.com/ipfs/go-ipfs-exchange-interface v0.2.1 // indirect
	github.com/ipfs/go-ipfs-util v0.0.3 // indirect
	github.com/ipfs/go-ipld-cbor v0.2.0 // indirect
	github.com/ipfs/go-ipld-format v0.6.0 // indirect
	github.com/ipfs/go-ipld-legacy v0.2.1 // indirect
	github.com/ipfs/go-log v1.0.5 // indirect
	github.com/ipfs/go-merkledag v0.11.0 // indirect
	github.com/ipfs/go-metrics-interface v0.0.1 // indirect
	github.com/ipfs/go-verifcid v0.0.3 // indirect
	github.com/ipld/go-car v0.6.2 // indirect
	github.com/ipld/go-codec-dagpb v1.6.0 // indirect
	github.com/jbenet/goprocess v0.1.4 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/libp2p/go-buffer-pool v0.1.0
	github.com/libp2p/go-libp2p-pubsub v0.13.0 // indirect
	github.com/libp2p/go-msgio v0.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/sha256-simd v1.0.1
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.1.0 // indirect
	github.com/multiformats/go-base36 v0.2.0 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multicodec v0.9.0
	github.com/multiformats/go-multistream v0.6.0 // indirect
	github.com/multiformats/go-varint v0.0.7 // indirect
	github.com/onsi/gomega v1.34.2 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.89.1-0.20231129105047-37766d95467a // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7 // indirect
	github.com/ucan-wg/go-ucan v0.0.0-20240916120445-37f52863156c // indirect
	github.com/whyrusleeping/cbor-gen v0.2.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.36.0
	golang.org/x/exp v0.0.0-20250128182459-e0ece0dbea4c // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	lukechampine.com/blake3 v1.3.0 // indirect
)

replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20240617180043-68d350f18fd4
