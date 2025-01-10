module github.com/google/deps.dev/examples/go/maven_parse_resolve

go 1.23.4

replace (
	deps.dev/util/maven => ../../../util/maven
	deps.dev/util/resolve => ../../../util/resolve
	deps.dev/util/semver => ../../../util/semver
)

require (
	deps.dev/api/v3alpha v0.0.0-20250109005846-cc10affbfdb9
	deps.dev/util/maven v0.0.0-20241203055422-1ee2cd4be494
	deps.dev/util/resolve v0.0.0-20240611045547-af20eef0f1eb
	google.golang.org/grpc v1.69.2
)

require (
	deps.dev/util/semver v0.0.0-20240109040450-1e316b822bc4 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
)
