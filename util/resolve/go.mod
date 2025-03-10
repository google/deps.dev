module deps.dev/util/resolve

go 1.23.4

replace (
	deps.dev/util/maven => ../maven
	deps.dev/util/pypi => ../pypi
	deps.dev/util/semver => ../semver
)

require (
	deps.dev/api/v3 v3.0.0-20240311054650-e1e6a3d70fb7
	deps.dev/util/maven v0.0.0-20240322043601-ff53416fec6a
	deps.dev/util/pypi v0.0.0-00010101000000-000000000000
	deps.dev/util/semver v0.0.0-20241230231135-52b7655a522f
	github.com/google/go-cmp v0.7.0
	google.golang.org/grpc v1.70.0
)

require (
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.35.2 // indirect
)
