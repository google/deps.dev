// Copyright 2024 Google LLC
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

package main

import (
	"context"

	"deps.dev/util/resolve"
)

type Client struct {
	resolve.APIClient
	versions     map[resolve.PackageKey]resolve.Version
	requirements map[resolve.VersionKey][]resolve.RequirementVersion
}

func NewOverrideClient(c resolve.APIClient) *Client {
	return &Client{
		APIClient:    c,
		versions:     make(map[resolve.PackageKey]resolve.Version),
		requirements: make(map[resolve.VersionKey][]resolve.RequirementVersion),
	}
}

func (c *Client) AddVersion(v resolve.Version, reqs []resolve.RequirementVersion) {
	c.versions[v.PackageKey] = v
	c.requirements[v.VersionKey] = append(c.requirements[v.VersionKey], reqs...)
}

func (c *Client) Version(ctx context.Context, vk resolve.VersionKey) (resolve.Version, error) {
	if v, ok := c.versions[vk.PackageKey]; ok {
		return v, nil
	}
	return c.APIClient.Version(ctx, vk)
}

func (c *Client) Versions(ctx context.Context, pk resolve.PackageKey) ([]resolve.Version, error) {
	return c.APIClient.Versions(ctx, pk)
}

func (c *Client) Requirements(ctx context.Context, vk resolve.VersionKey) ([]resolve.RequirementVersion, error) {
	if reqs, ok := c.requirements[vk]; ok {
		return reqs, nil
	}
	return c.APIClient.Requirements(ctx, vk)
}

func (c *Client) MatchingVersions(ctx context.Context, vk resolve.VersionKey) ([]resolve.Version, error) {
	return c.APIClient.MatchingVersions(ctx, vk)
}
