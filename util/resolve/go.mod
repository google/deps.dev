module deps.dev/util/resolve

go 1.21.1

replace deps.dev/util/semver => ../semver

require (
	deps.dev/api/v3alpha v0.0.0-20231114023923-e40c4d5c34e5
	deps.dev/util/semver v0.0.0-20240109040450-1e316b822bc4
	github.com/google/go-cmp v0.6.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.3 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)
