# createrepo_go

`createrepo_go` is a pure-Go toolkit for creating, reading, modifying, and
merging RPM repository metadata. It provides command-line tools compatible with
the familiar `createrepo_c`, `modifyrepo_c`, `mergerepo_c`, and `sqliterepo_c`
workflows, plus a Go package for embedding repository metadata operations in
other applications.

The Go module path is:

```text
github.com/rpm-software-management/createrepo_c
```

## Features

- Generate `repodata/repomd.xml` with `primary`, `filelists`, and `other`
  metadata from RPM packages.
- Preserve or add extra repository metadata such as comps, modules, and
  updateinfo records.
- Generate yum-compatible SQLite metadata databases.
- Merge multiple repositories into one output repository.
- Load local or HTTP(S) repository metadata from Go code.
- Read and write common metadata compression formats: `gz`, `bz2`, `xz`,
  `zstd`, and `zck`.
- Generate optional `filelists-ext` and delta RPM metadata.

## Requirements

- Go 1.24 or newer.

No C library bindings are required.

## Build

From the repository root:

```sh
go build ./...
```

To install all command-line tools into your configured Go binary directory:

```sh
go install ./cmd/...
```

This installs:

- `createrepo_c`
- `modifyrepo_c`
- `mergerepo_c`
- `sqliterepo_c`

## Command-Line Usage

Create repository metadata for a directory of RPMs:

```sh
createrepo_c /srv/repo
```

Write metadata to a separate output directory:

```sh
createrepo_c --outputdir /srv/repo-out /srv/rpms
```

Generate SQLite metadata along with XML metadata:

```sh
createrepo_c --database /srv/repo
```

Use a specific metadata compression format:

```sh
createrepo_c --compress-type gz /srv/repo
createrepo_c --zck /srv/repo
```

Update an existing repository while preserving additional metadata:

```sh
createrepo_c --update /srv/repo
```

Add a comps/group metadata file:

```sh
createrepo_c --groupfile comps.xml /srv/repo
```

Add or replace additional metadata in an existing `repodata` directory:

```sh
modifyrepo_c --mdtype modules modules.yaml /srv/repo/repodata
```

Remove an additional metadata record:

```sh
modifyrepo_c --remove modules /srv/repo/repodata
```

Merge repositories:

```sh
mergerepo_c --repo /srv/repo-a --repo /srv/repo-b --outputdir /srv/merged
```

Generate SQLite metadata for an existing repository:

```sh
sqliterepo_c /srv/repo
```

Each command supports `--version`. Unsupported compatibility flags are accepted
where useful so existing command lines can migrate gradually.

## Go API

Import the public package:

```go
import "github.com/rpm-software-management/createrepo_c/pkg/createrepo"
```

Create repository metadata:

```go
err := createrepo.Create(ctx, createrepo.Options{
	Directory:   "/srv/repo",
	Database:    true,
	Compression: createrepo.CompressionZstd,
	Checksum:    createrepo.ChecksumSHA256,
})
```

Load repository metadata:

```go
repo, err := createrepo.LoadMetadata(ctx, "https://example.com/repo/", createrepo.LoadOptions{
	VerifyChecksums: true,
})
if err != nil {
	return err
}

for _, pkg := range repo.Packages {
	fmt.Println(pkg.Name, pkg.Version, pkg.Arch)
}
```

Modify an existing `repodata` directory:

```go
err := createrepo.Modify(ctx, createrepo.ModifyOptions{
	MetadataPath: "/tmp/modules.yaml",
	RepodataDir:  "/srv/repo/repodata",
	MetadataType: "modules",
	Compress:     true,
	Compression:  createrepo.CompressionZstd,
})
```

## Supported Metadata

The package can parse and emit the standard package metadata files:

- `primary.xml`
- `filelists.xml`
- `filelists-ext.xml`
- `other.xml`
- `repomd.xml`
- `updateinfo.xml`

Additional metadata records are preserved through create/update flows when
requested.

## Development

Run the test suite with:

```sh
go test ./...
```

Useful package layout:

- `cmd/` contains the command-line entrypoints.
- `internal/cli/` contains flag parsing and command dispatch.
- `pkg/createrepo/` contains the public Go API and metadata implementation.
- `tests/testdata/` contains RPMs and repository fixtures used by tests.

## License

This project is distributed under the GNU General Public License version 2. See
[`LICENSE`](LICENSE) for the full license text.
