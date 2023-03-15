# o11y-deps-installer

o11y-deps-installer is a command-line tool that installs the necessary
dependencies for the O11y project. It simplifies the installation
process by bundling the required libraries and tools into a single, easy-to-use
executable.

## Features

- Extracts embedded dependencies from a tar.gz file
- Updates shebangs and symlinks to match the target installation directory
- Fixes binary and library dependencies using patchelf
- Supports uninstallation, reinstallation, and forced installation options

## Prerequisites

Go 1.20 or higher

## Building

To build the o11y-deps-installer, use the provided Makefile:

```sh
make download_packer
make build_packer_image
make build
```

## Usage

To install the dependencies, simply run the generated `o11y-deps-installer` binary:

```sh
./o11y-deps-installer
```

By default, the dependencies will be installed into `/opt/o11y/deps`. You can change the destination directory using the `--deps-home` flag:

```sh
./o11y-deps-installer --deps-home /path/to/custom/directory
```

To uninstall the dependencies, use the `--uninstall` flag:

```sh
./o11y-deps-installer --uninstall
```

To reinstall the dependencies, use the `--reinstall` flag:

```sh
./o11y-deps-installer --reinstall
```

## License

The o11y-deps-installer source code is released under the [Apache License 2.0](./LICENSE).

Please note that the release artifacts, which include the bundled dependencies
like Python, Alpine Linux, Ansible, and others, are subject to their respective
licenses. These dependencies are not covered by the Apache License 2.0 of
o11y-deps-installer.
