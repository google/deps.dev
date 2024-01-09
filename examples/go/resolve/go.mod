module github.com/google/deps.dev/examples/go/resolve

go 1.21.1

replace (
	deps.dev/util/resolve => ../../../util/resolve
	deps.dev/util/semver => ../../../util/semver
)

require (
	deps.dev/api/v3alpha v0.0.0-20231114023923-e40c4d5c34e5
	deps.dev/util/resolve v0.0.0-20240109042120-d4545400844a
	google.golang.org/grpc v1.59.0
)

require (
	deps.dev/util/semver v0.0.0-20240109040450-1e316b822bc4 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
