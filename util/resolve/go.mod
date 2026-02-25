module deps.dev/util/resolve

go 1.24.0

replace (
	deps.dev/util/maven => ../maven
	deps.dev/util/pypi => ../pypi
	deps.dev/util/semver => ../semver
)

require (
	deps.dev/api/v3 v3.0.0-20260225062937-bb3cf65ba738
	deps.dev/util/maven v0.0.0-20240322043601-ff53416fec6a
	deps.dev/util/pypi v0.0.0-20250307021655-d811e36f9cad
	deps.dev/util/semver v0.0.0-20241230231135-52b7655a522f
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8
	github.com/google/go-cmp v0.7.0
	google.golang.org/grpc v1.78.0
)

require (
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
