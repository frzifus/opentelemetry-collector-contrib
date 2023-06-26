// Deprecated: use github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector instead.
module github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor

go 1.19

require (
	github.com/hashicorp/golang-lru v0.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.79.0
	github.com/stretchr/testify v1.8.4
	github.com/tilinna/clock v1.1.0
	go.opentelemetry.io/collector v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/component v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/confmap v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/consumer v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/exporter v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/exporter/otlpexporter v0.79.1-0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230609201858-ed8547a8e5d6
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0013
	go.opentelemetry.io/collector/processor v0.0.0-20230609193203-89d1060c7606
	go.opentelemetry.io/collector/semconv v0.79.1-0.20230609201858-ed8547a8e5d6
	go.uber.org/zap v1.24.0
	google.golang.org/grpc v1.56.0
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.1.18 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/extension v0.0.0-20230609200026-525adf4a682a // indirect
	go.opentelemetry.io/collector/receiver v0.79.1-0.20230609201858-ed8547a8e5d6 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.42.0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest
