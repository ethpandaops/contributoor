module github.com/ethpandaops/contributoor

go 1.25.1

// release-xatu branch.
replace github.com/probe-lab/hermes => github.com/ethpandaops/hermes v0.0.4-0.20251001001342-3643aa32bc73

require (
	buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go v1.36.11-20260209202127-80ab13bee0bf.1
	buf.build/go/protovalidate v1.1.2
	github.com/attestantio/go-eth2-client v0.28.0
	github.com/beevik/ntp v1.5.0
	github.com/creasty/defaults v1.8.0
	github.com/ethpandaops/beacon v0.67.0
	github.com/ethpandaops/ethcore v0.0.0-20260112064422-e7fe02956738
	github.com/ethpandaops/ethereum-package-go v0.8.2-0.20260218210947-b86bea76c8e0
	github.com/ethpandaops/ethwallclock v0.4.0
	github.com/ethpandaops/xatu v1.8.9
	github.com/go-co-op/gocron/v2 v2.19.1
	github.com/google/uuid v1.6.0
	github.com/jellydator/ttlcache/v3 v3.4.0
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.23.2
	github.com/sirupsen/logrus v1.9.4
	github.com/ssgreg/journalhook v0.0.0-20180529133218-9a0802d16187
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli/v2 v2.27.7
	go.uber.org/mock v0.6.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/IBM/sarama v1.46.3 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/OffchainLabs/go-bitfield v0.0.0-20251031151322-f427d04d8506 // indirect
	github.com/ProjectZKM/Ziren/crates/go-runtime/zkvm_runtime v0.0.0-20260218040609-6f1c0c95351b // indirect
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/casbin/govaluate v1.10.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chuckpreslar/emission v0.0.0-20170206194824-a7ddd980baf9 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/emicklei/dot v1.11.0 // indirect
	github.com/ethereum/go-ethereum v1.17.0 // indirect
	github.com/ferranbt/fastssz v1.0.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-co-op/gocron v1.37.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-yaml/yaml v2.1.0+incompatible // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/cel-go v0.27.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.3 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/huandu/go-assert v1.1.6 // indirect
	github.com/huandu/go-clone v1.7.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/kurtosis-tech/kurtosis-portal/api/golang v0.0.0-20231031173452-349f1ec9a443 // indirect
	github.com/kurtosis-tech/kurtosis/api/golang v1.15.2 // indirect
	github.com/kurtosis-tech/kurtosis/contexts-config-store v0.0.0-20260210032812-a84896be5b94 // indirect
	github.com/kurtosis-tech/kurtosis/grpc-file-transfer/golang v0.0.0-20260210032812-a84896be5b94 // indirect
	github.com/kurtosis-tech/kurtosis/path-compression v0.0.0-20260210032812-a84896be5b94 // indirect
	github.com/kurtosis-tech/stacktrace v0.0.0-20211028211901-1c67a77b5409 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mholt/archiver v3.1.1+incompatible // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/onsi/gomega v1.39.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/pk910/dynamic-ssz v1.2.1 // indirect
	github.com/pk910/hashtree-bindings v0.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/r3labs/sse/v2 v2.10.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/ssgreg/journald v1.0.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xrash/smetrics v0.0.0-20250705151800-55b8f293f342 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.40.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/grpc v1.79.1 // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
