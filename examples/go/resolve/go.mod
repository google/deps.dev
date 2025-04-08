module github.com/google/deps.dev/examples/go/resolve

go 1.23.4

replace (
	deps.dev/util/maven => ../../../util/maven
	deps.dev/util/resolve => ../../../util/resolve
	deps.dev/util/semver => ../../../util/semver
)

require (
	deps.dev/api/v3 v3.0.0-20240311054650-e1e6a3d70fb7
	deps.dev/util/resolve v0.0.0-20240312000934-38ffc8dd1d92
	google.golang.org/grpc v1.71.1
)

require (
	deps.dev/util/maven v0.0.0-20241203055422-1ee2cd4be494 // indirect
	deps.dev/util/semver v0.0.0-20241230231135-52b7655a522f // indirect
	golang.org/x/net v0.36.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
)
