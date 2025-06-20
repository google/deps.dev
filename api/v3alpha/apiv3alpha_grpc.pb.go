// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v5.29.5
// source: apiv3alpha.proto

package v3alpha

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	Insights_GetPackage_FullMethodName                = "/deps_dev.v3alpha.Insights/GetPackage"
	Insights_GetVersion_FullMethodName                = "/deps_dev.v3alpha.Insights/GetVersion"
	Insights_GetVersionBatch_FullMethodName           = "/deps_dev.v3alpha.Insights/GetVersionBatch"
	Insights_GetRequirements_FullMethodName           = "/deps_dev.v3alpha.Insights/GetRequirements"
	Insights_GetDependencies_FullMethodName           = "/deps_dev.v3alpha.Insights/GetDependencies"
	Insights_GetDependents_FullMethodName             = "/deps_dev.v3alpha.Insights/GetDependents"
	Insights_GetCapabilities_FullMethodName           = "/deps_dev.v3alpha.Insights/GetCapabilities"
	Insights_GetProject_FullMethodName                = "/deps_dev.v3alpha.Insights/GetProject"
	Insights_GetProjectBatch_FullMethodName           = "/deps_dev.v3alpha.Insights/GetProjectBatch"
	Insights_GetProjectPackageVersions_FullMethodName = "/deps_dev.v3alpha.Insights/GetProjectPackageVersions"
	Insights_GetAdvisory_FullMethodName               = "/deps_dev.v3alpha.Insights/GetAdvisory"
	Insights_GetSimilarlyNamedPackages_FullMethodName = "/deps_dev.v3alpha.Insights/GetSimilarlyNamedPackages"
	Insights_Query_FullMethodName                     = "/deps_dev.v3alpha.Insights/Query"
	Insights_PurlLookup_FullMethodName                = "/deps_dev.v3alpha.Insights/PurlLookup"
	Insights_PurlLookupBatch_FullMethodName           = "/deps_dev.v3alpha.Insights/PurlLookupBatch"
	Insights_QueryContainerImages_FullMethodName      = "/deps_dev.v3alpha.Insights/QueryContainerImages"
)

// InsightsClient is the client API for Insights service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type InsightsClient interface {
	// GetPackage returns information about a package, including a list of its
	// available versions, with the default version marked if known.
	GetPackage(ctx context.Context, in *GetPackageRequest, opts ...grpc.CallOption) (*Package, error)
	// GetVersion returns information about a specific package version, including
	// its licenses and any security advisories known to affect it.
	GetVersion(ctx context.Context, in *GetVersionRequest, opts ...grpc.CallOption) (*Version, error)
	// GetVersionBatch performs GetVersion requests for a batch of versions.
	// Large result sets may be paginated.
	GetVersionBatch(ctx context.Context, in *GetVersionBatchRequest, opts ...grpc.CallOption) (*VersionBatch, error)
	// GetRequirements returns the requirements for a given version in a
	// system-specific format. Requirements are currently available for
	// Maven, npm and NuGet.
	//
	// Requirements are the dependency constraints specified by the version.
	GetRequirements(ctx context.Context, in *GetRequirementsRequest, opts ...grpc.CallOption) (*Requirements, error)
	// GetDependencies returns a resolved dependency graph for the given package
	// version. Dependencies are currently available for Go, npm, Cargo, Maven
	// and PyPI.
	//
	// Dependencies are the resolution of the requirements (dependency
	// constraints) specified by a version.
	//
	// The dependency graph should be similar to one produced by installing the
	// package version on a generic 64-bit Linux system, with no other
	// dependencies present. The precise meaning of this varies from system to
	// system.
	GetDependencies(ctx context.Context, in *GetDependenciesRequest, opts ...grpc.CallOption) (*Dependencies, error)
	// GetDependents returns information about the number of distinct packages
	// known to depend on the given package version. Dependent counts are
	// currently available for Go, npm, Cargo, Maven and PyPI.
	//
	// Dependent counts are derived from the dependency graphs computed by
	// deps.dev, which means that only public dependents are counted. As such,
	// dependent counts should be treated as indicative of relative popularity
	// rather than precisely accurate.
	GetDependents(ctx context.Context, in *GetDependentsRequest, opts ...grpc.CallOption) (*Dependents, error)
	// GetCapabilityRequest returns counts for direct and indirect calls to
	// Capslock capabilities for a given package version.
	// Currently only available for Go.
	GetCapabilities(ctx context.Context, in *GetCapabilitiesRequest, opts ...grpc.CallOption) (*Capabilities, error)
	// GetProject returns information about projects hosted by GitHub, GitLab, or
	// BitBucket, when known to us.
	GetProject(ctx context.Context, in *GetProjectRequest, opts ...grpc.CallOption) (*Project, error)
	// GetProjectBatch performs GetProjectBatch requests for a batch of projects.
	// Large result sets may be paginated.
	GetProjectBatch(ctx context.Context, in *GetProjectBatchRequest, opts ...grpc.CallOption) (*ProjectBatch, error)
	// GetProjectPackageVersions returns known mappings between the requested
	// project and package versions.
	// At most 1500 package versions are returned. Mappings which were derived
	// from attestations are served first.
	GetProjectPackageVersions(ctx context.Context, in *GetProjectPackageVersionsRequest, opts ...grpc.CallOption) (*ProjectPackageVersions, error)
	// GetAdvisory returns information about security advisories hosted by OSV.
	GetAdvisory(ctx context.Context, in *GetAdvisoryRequest, opts ...grpc.CallOption) (*Advisory, error)
	// GetSimilarlyNamedPackages returns packages with names that are similar to
	// the requested package. This similarity relation is computed by deps.dev.
	GetSimilarlyNamedPackages(ctx context.Context, in *GetSimilarlyNamedPackagesRequest, opts ...grpc.CallOption) (*SimilarlyNamedPackages, error)
	// Query returns information about multiple package versions, which can be
	// specified by name, content hash, or both. If a hash was specified in the
	// request, it returns the artifacts that matched the hash.
	//
	// Querying by content hash is currently supported for npm, Cargo, Maven,
	// NuGet, PyPI and RubyGems. It is typical for hash queries to return many
	// results; hashes are matched against multiple release artifacts (such as
	// JAR files) that comprise package versions, and any given artifact may
	// appear in several package versions.
	Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResult, error)
	// PurlLookup searches for a package or package version specified via
	// [purl](https://github.com/package-url/purl-spec),
	// and returns the corresponding result from GetPackage or GetVersion as appropriate.
	//
	// For a package lookup, the purl should be in the form
	//
	//	`pkg:type/namespace/name`   for a namespaced package name, or
	//	`pkg:type/name`             for a non-namespaced package name.
	//
	// For a package version lookup, the purl should be in the form
	//
	//	`pkg:type/namespace/name@version`, or
	//	`pkg:type/name@version`.
	//
	// Extra fields in the purl must be empty, otherwise the request will fail.
	// In particular, there must be no subpath or qualifiers.
	//
	// Supported values for `type` are `cargo`, `golang`, `maven`, `npm`, `nuget`
	// and `pypi`. Further details on types, and how to form purls of each type,
	// can be found in the
	// [purl spec](https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst).
	//
	// Special characters in purls must be percent-encoded. This is described in
	// detail by the
	// [purl spec](https://github.com/package-url/purl-spec/blob/master/PURL-SPECIFICATION.rst).
	PurlLookup(ctx context.Context, in *PurlLookupRequest, opts ...grpc.CallOption) (*PurlLookupResult, error)
	// PurlLookupBatch performs PurlLookup requests for a batch of purls.
	// This endpoint only supports version lookups. Purls in requests
	// must include a version field.
	//
	// Supported purl forms are
	//
	//	`pkg:type/namespace/name@version` for a namespaced package name, or
	//	`pkg:type/name@version`           for a non-namespaced package name.
	//
	// Extra fields in the purl must be empty, otherwise the request will fail.
	// In particular, there must be no subpath or qualifiers.
	//
	// Large result sets may be paginated.
	PurlLookupBatch(ctx context.Context, in *PurlLookupBatchRequest, opts ...grpc.CallOption) (*PurlLookupBatchResult, error)
	// QueryContainerImages searches for container image repositories on
	// DockerHub that match the requested OCI Chain ID. At most 1000 image
	// repositories are returned.
	//
	// An image repository is identifier (eg. 'tensorflow') that refers to
	// a collection of images.
	//
	// An OCI Chain ID is a hashed encoding of an ordered sequence of OCI
	// layers. For further details see the [OCI Chain ID
	// spec](https://github.com/opencontainers/image-spec/blob/main/config.md#layer-chainid).
	QueryContainerImages(ctx context.Context, in *QueryContainerImagesRequest, opts ...grpc.CallOption) (*QueryContainerImagesResult, error)
}

type insightsClient struct {
	cc grpc.ClientConnInterface
}

func NewInsightsClient(cc grpc.ClientConnInterface) InsightsClient {
	return &insightsClient{cc}
}

func (c *insightsClient) GetPackage(ctx context.Context, in *GetPackageRequest, opts ...grpc.CallOption) (*Package, error) {
	out := new(Package)
	err := c.cc.Invoke(ctx, Insights_GetPackage_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetVersion(ctx context.Context, in *GetVersionRequest, opts ...grpc.CallOption) (*Version, error) {
	out := new(Version)
	err := c.cc.Invoke(ctx, Insights_GetVersion_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetVersionBatch(ctx context.Context, in *GetVersionBatchRequest, opts ...grpc.CallOption) (*VersionBatch, error) {
	out := new(VersionBatch)
	err := c.cc.Invoke(ctx, Insights_GetVersionBatch_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetRequirements(ctx context.Context, in *GetRequirementsRequest, opts ...grpc.CallOption) (*Requirements, error) {
	out := new(Requirements)
	err := c.cc.Invoke(ctx, Insights_GetRequirements_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetDependencies(ctx context.Context, in *GetDependenciesRequest, opts ...grpc.CallOption) (*Dependencies, error) {
	out := new(Dependencies)
	err := c.cc.Invoke(ctx, Insights_GetDependencies_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetDependents(ctx context.Context, in *GetDependentsRequest, opts ...grpc.CallOption) (*Dependents, error) {
	out := new(Dependents)
	err := c.cc.Invoke(ctx, Insights_GetDependents_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetCapabilities(ctx context.Context, in *GetCapabilitiesRequest, opts ...grpc.CallOption) (*Capabilities, error) {
	out := new(Capabilities)
	err := c.cc.Invoke(ctx, Insights_GetCapabilities_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetProject(ctx context.Context, in *GetProjectRequest, opts ...grpc.CallOption) (*Project, error) {
	out := new(Project)
	err := c.cc.Invoke(ctx, Insights_GetProject_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetProjectBatch(ctx context.Context, in *GetProjectBatchRequest, opts ...grpc.CallOption) (*ProjectBatch, error) {
	out := new(ProjectBatch)
	err := c.cc.Invoke(ctx, Insights_GetProjectBatch_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetProjectPackageVersions(ctx context.Context, in *GetProjectPackageVersionsRequest, opts ...grpc.CallOption) (*ProjectPackageVersions, error) {
	out := new(ProjectPackageVersions)
	err := c.cc.Invoke(ctx, Insights_GetProjectPackageVersions_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetAdvisory(ctx context.Context, in *GetAdvisoryRequest, opts ...grpc.CallOption) (*Advisory, error) {
	out := new(Advisory)
	err := c.cc.Invoke(ctx, Insights_GetAdvisory_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) GetSimilarlyNamedPackages(ctx context.Context, in *GetSimilarlyNamedPackagesRequest, opts ...grpc.CallOption) (*SimilarlyNamedPackages, error) {
	out := new(SimilarlyNamedPackages)
	err := c.cc.Invoke(ctx, Insights_GetSimilarlyNamedPackages_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResult, error) {
	out := new(QueryResult)
	err := c.cc.Invoke(ctx, Insights_Query_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) PurlLookup(ctx context.Context, in *PurlLookupRequest, opts ...grpc.CallOption) (*PurlLookupResult, error) {
	out := new(PurlLookupResult)
	err := c.cc.Invoke(ctx, Insights_PurlLookup_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) PurlLookupBatch(ctx context.Context, in *PurlLookupBatchRequest, opts ...grpc.CallOption) (*PurlLookupBatchResult, error) {
	out := new(PurlLookupBatchResult)
	err := c.cc.Invoke(ctx, Insights_PurlLookupBatch_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *insightsClient) QueryContainerImages(ctx context.Context, in *QueryContainerImagesRequest, opts ...grpc.CallOption) (*QueryContainerImagesResult, error) {
	out := new(QueryContainerImagesResult)
	err := c.cc.Invoke(ctx, Insights_QueryContainerImages_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// InsightsServer is the server API for Insights service.
// All implementations must embed UnimplementedInsightsServer
// for forward compatibility
type InsightsServer interface {
	// GetPackage returns information about a package, including a list of its
	// available versions, with the default version marked if known.
	GetPackage(context.Context, *GetPackageRequest) (*Package, error)
	// GetVersion returns information about a specific package version, including
	// its licenses and any security advisories known to affect it.
	GetVersion(context.Context, *GetVersionRequest) (*Version, error)
	// GetVersionBatch performs GetVersion requests for a batch of versions.
	// Large result sets may be paginated.
	GetVersionBatch(context.Context, *GetVersionBatchRequest) (*VersionBatch, error)
	// GetRequirements returns the requirements for a given version in a
	// system-specific format. Requirements are currently available for
	// Maven, npm and NuGet.
	//
	// Requirements are the dependency constraints specified by the version.
	GetRequirements(context.Context, *GetRequirementsRequest) (*Requirements, error)
	// GetDependencies returns a resolved dependency graph for the given package
	// version. Dependencies are currently available for Go, npm, Cargo, Maven
	// and PyPI.
	//
	// Dependencies are the resolution of the requirements (dependency
	// constraints) specified by a version.
	//
	// The dependency graph should be similar to one produced by installing the
	// package version on a generic 64-bit Linux system, with no other
	// dependencies present. The precise meaning of this varies from system to
	// system.
	GetDependencies(context.Context, *GetDependenciesRequest) (*Dependencies, error)
	// GetDependents returns information about the number of distinct packages
	// known to depend on the given package version. Dependent counts are
	// currently available for Go, npm, Cargo, Maven and PyPI.
	//
	// Dependent counts are derived from the dependency graphs computed by
	// deps.dev, which means that only public dependents are counted. As such,
	// dependent counts should be treated as indicative of relative popularity
	// rather than precisely accurate.
	GetDependents(context.Context, *GetDependentsRequest) (*Dependents, error)
	// GetCapabilityRequest returns counts for direct and indirect calls to
	// Capslock capabilities for a given package version.
	// Currently only available for Go.
	GetCapabilities(context.Context, *GetCapabilitiesRequest) (*Capabilities, error)
	// GetProject returns information about projects hosted by GitHub, GitLab, or
	// BitBucket, when known to us.
	GetProject(context.Context, *GetProjectRequest) (*Project, error)
	// GetProjectBatch performs GetProjectBatch requests for a batch of projects.
	// Large result sets may be paginated.
	GetProjectBatch(context.Context, *GetProjectBatchRequest) (*ProjectBatch, error)
	// GetProjectPackageVersions returns known mappings between the requested
	// project and package versions.
	// At most 1500 package versions are returned. Mappings which were derived
	// from attestations are served first.
	GetProjectPackageVersions(context.Context, *GetProjectPackageVersionsRequest) (*ProjectPackageVersions, error)
	// GetAdvisory returns information about security advisories hosted by OSV.
	GetAdvisory(context.Context, *GetAdvisoryRequest) (*Advisory, error)
	// GetSimilarlyNamedPackages returns packages with names that are similar to
	// the requested package. This similarity relation is computed by deps.dev.
	GetSimilarlyNamedPackages(context.Context, *GetSimilarlyNamedPackagesRequest) (*SimilarlyNamedPackages, error)
	// Query returns information about multiple package versions, which can be
	// specified by name, content hash, or both. If a hash was specified in the
	// request, it returns the artifacts that matched the hash.
	//
	// Querying by content hash is currently supported for npm, Cargo, Maven,
	// NuGet, PyPI and RubyGems. It is typical for hash queries to return many
	// results; hashes are matched against multiple release artifacts (such as
	// JAR files) that comprise package versions, and any given artifact may
	// appear in several package versions.
	Query(context.Context, *QueryRequest) (*QueryResult, error)
	// PurlLookup searches for a package or package version specified via
	// [purl](https://github.com/package-url/purl-spec),
	// and returns the corresponding result from GetPackage or GetVersion as appropriate.
	//
	// For a package lookup, the purl should be in the form
	//
	//	`pkg:type/namespace/name`   for a namespaced package name, or
	//	`pkg:type/name`             for a non-namespaced package name.
	//
	// For a package version lookup, the purl should be in the form
	//
	//	`pkg:type/namespace/name@version`, or
	//	`pkg:type/name@version`.
	//
	// Extra fields in the purl must be empty, otherwise the request will fail.
	// In particular, there must be no subpath or qualifiers.
	//
	// Supported values for `type` are `cargo`, `golang`, `maven`, `npm`, `nuget`
	// and `pypi`. Further details on types, and how to form purls of each type,
	// can be found in the
	// [purl spec](https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst).
	//
	// Special characters in purls must be percent-encoded. This is described in
	// detail by the
	// [purl spec](https://github.com/package-url/purl-spec/blob/master/PURL-SPECIFICATION.rst).
	PurlLookup(context.Context, *PurlLookupRequest) (*PurlLookupResult, error)
	// PurlLookupBatch performs PurlLookup requests for a batch of purls.
	// This endpoint only supports version lookups. Purls in requests
	// must include a version field.
	//
	// Supported purl forms are
	//
	//	`pkg:type/namespace/name@version` for a namespaced package name, or
	//	`pkg:type/name@version`           for a non-namespaced package name.
	//
	// Extra fields in the purl must be empty, otherwise the request will fail.
	// In particular, there must be no subpath or qualifiers.
	//
	// Large result sets may be paginated.
	PurlLookupBatch(context.Context, *PurlLookupBatchRequest) (*PurlLookupBatchResult, error)
	// QueryContainerImages searches for container image repositories on
	// DockerHub that match the requested OCI Chain ID. At most 1000 image
	// repositories are returned.
	//
	// An image repository is identifier (eg. 'tensorflow') that refers to
	// a collection of images.
	//
	// An OCI Chain ID is a hashed encoding of an ordered sequence of OCI
	// layers. For further details see the [OCI Chain ID
	// spec](https://github.com/opencontainers/image-spec/blob/main/config.md#layer-chainid).
	QueryContainerImages(context.Context, *QueryContainerImagesRequest) (*QueryContainerImagesResult, error)
	mustEmbedUnimplementedInsightsServer()
}

// UnimplementedInsightsServer must be embedded to have forward compatible implementations.
type UnimplementedInsightsServer struct {
}

func (UnimplementedInsightsServer) GetPackage(context.Context, *GetPackageRequest) (*Package, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPackage not implemented")
}
func (UnimplementedInsightsServer) GetVersion(context.Context, *GetVersionRequest) (*Version, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersion not implemented")
}
func (UnimplementedInsightsServer) GetVersionBatch(context.Context, *GetVersionBatchRequest) (*VersionBatch, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetVersionBatch not implemented")
}
func (UnimplementedInsightsServer) GetRequirements(context.Context, *GetRequirementsRequest) (*Requirements, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequirements not implemented")
}
func (UnimplementedInsightsServer) GetDependencies(context.Context, *GetDependenciesRequest) (*Dependencies, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDependencies not implemented")
}
func (UnimplementedInsightsServer) GetDependents(context.Context, *GetDependentsRequest) (*Dependents, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDependents not implemented")
}
func (UnimplementedInsightsServer) GetCapabilities(context.Context, *GetCapabilitiesRequest) (*Capabilities, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCapabilities not implemented")
}
func (UnimplementedInsightsServer) GetProject(context.Context, *GetProjectRequest) (*Project, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProject not implemented")
}
func (UnimplementedInsightsServer) GetProjectBatch(context.Context, *GetProjectBatchRequest) (*ProjectBatch, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProjectBatch not implemented")
}
func (UnimplementedInsightsServer) GetProjectPackageVersions(context.Context, *GetProjectPackageVersionsRequest) (*ProjectPackageVersions, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetProjectPackageVersions not implemented")
}
func (UnimplementedInsightsServer) GetAdvisory(context.Context, *GetAdvisoryRequest) (*Advisory, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAdvisory not implemented")
}
func (UnimplementedInsightsServer) GetSimilarlyNamedPackages(context.Context, *GetSimilarlyNamedPackagesRequest) (*SimilarlyNamedPackages, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSimilarlyNamedPackages not implemented")
}
func (UnimplementedInsightsServer) Query(context.Context, *QueryRequest) (*QueryResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedInsightsServer) PurlLookup(context.Context, *PurlLookupRequest) (*PurlLookupResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PurlLookup not implemented")
}
func (UnimplementedInsightsServer) PurlLookupBatch(context.Context, *PurlLookupBatchRequest) (*PurlLookupBatchResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PurlLookupBatch not implemented")
}
func (UnimplementedInsightsServer) QueryContainerImages(context.Context, *QueryContainerImagesRequest) (*QueryContainerImagesResult, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryContainerImages not implemented")
}
func (UnimplementedInsightsServer) mustEmbedUnimplementedInsightsServer() {}

// UnsafeInsightsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to InsightsServer will
// result in compilation errors.
type UnsafeInsightsServer interface {
	mustEmbedUnimplementedInsightsServer()
}

func RegisterInsightsServer(s grpc.ServiceRegistrar, srv InsightsServer) {
	s.RegisterService(&Insights_ServiceDesc, srv)
}

func _Insights_GetPackage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetPackageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetPackage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetPackage_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetPackage(ctx, req.(*GetPackageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetVersion_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetVersion(ctx, req.(*GetVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetVersionBatch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetVersionBatchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetVersionBatch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetVersionBatch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetVersionBatch(ctx, req.(*GetVersionBatchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetRequirements_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRequirementsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetRequirements(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetRequirements_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetRequirements(ctx, req.(*GetRequirementsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetDependencies_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDependenciesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetDependencies(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetDependencies_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetDependencies(ctx, req.(*GetDependenciesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetDependents_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetDependentsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetDependents(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetDependents_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetDependents(ctx, req.(*GetDependentsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetCapabilities_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCapabilitiesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetCapabilities(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetCapabilities_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetCapabilities(ctx, req.(*GetCapabilitiesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetProject_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetProject(ctx, req.(*GetProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetProjectBatch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetProjectBatchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetProjectBatch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetProjectBatch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetProjectBatch(ctx, req.(*GetProjectBatchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetProjectPackageVersions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetProjectPackageVersionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetProjectPackageVersions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetProjectPackageVersions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetProjectPackageVersions(ctx, req.(*GetProjectPackageVersionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetAdvisory_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAdvisoryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetAdvisory(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetAdvisory_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetAdvisory(ctx, req.(*GetAdvisoryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_GetSimilarlyNamedPackages_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSimilarlyNamedPackagesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).GetSimilarlyNamedPackages(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_GetSimilarlyNamedPackages_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).GetSimilarlyNamedPackages(ctx, req.(*GetSimilarlyNamedPackagesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_Query_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).Query(ctx, req.(*QueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_PurlLookup_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PurlLookupRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).PurlLookup(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_PurlLookup_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).PurlLookup(ctx, req.(*PurlLookupRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_PurlLookupBatch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PurlLookupBatchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).PurlLookupBatch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_PurlLookupBatch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).PurlLookupBatch(ctx, req.(*PurlLookupBatchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Insights_QueryContainerImages_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryContainerImagesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(InsightsServer).QueryContainerImages(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Insights_QueryContainerImages_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(InsightsServer).QueryContainerImages(ctx, req.(*QueryContainerImagesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Insights_ServiceDesc is the grpc.ServiceDesc for Insights service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Insights_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "deps_dev.v3alpha.Insights",
	HandlerType: (*InsightsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetPackage",
			Handler:    _Insights_GetPackage_Handler,
		},
		{
			MethodName: "GetVersion",
			Handler:    _Insights_GetVersion_Handler,
		},
		{
			MethodName: "GetVersionBatch",
			Handler:    _Insights_GetVersionBatch_Handler,
		},
		{
			MethodName: "GetRequirements",
			Handler:    _Insights_GetRequirements_Handler,
		},
		{
			MethodName: "GetDependencies",
			Handler:    _Insights_GetDependencies_Handler,
		},
		{
			MethodName: "GetDependents",
			Handler:    _Insights_GetDependents_Handler,
		},
		{
			MethodName: "GetCapabilities",
			Handler:    _Insights_GetCapabilities_Handler,
		},
		{
			MethodName: "GetProject",
			Handler:    _Insights_GetProject_Handler,
		},
		{
			MethodName: "GetProjectBatch",
			Handler:    _Insights_GetProjectBatch_Handler,
		},
		{
			MethodName: "GetProjectPackageVersions",
			Handler:    _Insights_GetProjectPackageVersions_Handler,
		},
		{
			MethodName: "GetAdvisory",
			Handler:    _Insights_GetAdvisory_Handler,
		},
		{
			MethodName: "GetSimilarlyNamedPackages",
			Handler:    _Insights_GetSimilarlyNamedPackages_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _Insights_Query_Handler,
		},
		{
			MethodName: "PurlLookup",
			Handler:    _Insights_PurlLookup_Handler,
		},
		{
			MethodName: "PurlLookupBatch",
			Handler:    _Insights_PurlLookupBatch_Handler,
		},
		{
			MethodName: "QueryContainerImages",
			Handler:    _Insights_QueryContainerImages_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "apiv3alpha.proto",
}
