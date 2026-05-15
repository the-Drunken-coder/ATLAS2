module github.com/anomalyco/atlas-core

go 1.26.2

require (
	atlas.local/protocol v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.9.2
	golang.org/x/sys v0.43.0
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.10
)

replace atlas.local/protocol => ../atlas-protocol/packages/go

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.1 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)
