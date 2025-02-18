# semver

The semver package implements parsing, ordering, and matching of versions, as
defined by [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html),
with support for extensions and quirks implemented by a number of package
management systems, including:
- Cargo
- Go
- Maven
- NPM
- NuGet
- PyPI
- RubyGems
- Composer

For a detailed description of what is supported, see `version.go`.

## Example usage

Determining which of two versions is greater:

```
v1, err := semver.NPM.Parse("1.2.3")
v2, err := semver.NPM.Parse("2.3.4")
if v1.Compare(v2) < 0 { ... }
```

Determining whether a version satisfies a constraint:

```
v, err := semver.NPM.Parse("1.2.3")
c, err := semver.NPM.ParseConstraint("^1.0.0")
if c.MatchVersion(v) { ... }
```
