module github.com/yourorg/hsm

go 1.21

require (
	github.com/google/uuid v1.6.0
	github.com/yourorg/hsm/api v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v1.64.0
)

require (
	github.com/stretchr/testify v1.8.2 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace github.com/yourorg/hsm/api => ./api
