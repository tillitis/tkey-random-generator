[![ci](https://github.com/tillitis/tkey-random-generator/actions/workflows/ci.yaml/badge.svg?event=push)](https://github.com/tillitis/tkey-random-generator/actions/workflows/ci.yaml)

# TKey Random Generator

`tkey-random-generator` is a hardware-backed source of high-quality
random numbers.

The generator itself, the `random-generator` device app, is using a
construction similar to the NIST Hash\_DRBG ([NIST SP
800-90A](https://csrc.nist.gov/pubs/sp/800/90/a/r1/final)), but based
on the BLAKE2s hash function. The internal 512-bit internal state is
periodically reseeded using the TKey True Random Number Generator
(TRNG) entropy source.

One unique feature is the ability to provide a cryptographic signature
of the data generated. Since the keypair used for signing is based on
the UDS, the application, and any USS supplied, a user verifying the
signature can trust that the received data was generated on the TKey
by the application loaded and has not been tampered with since.

It is always recommended to use [the latest
release](https://github.com/tillitis/tkey-random-generator/releases/)).
There you can also find some pre-compiled binaries.

## Signed Random Data

If used with `tkey-random-generator generate -s` then
`random-generator` will sign the produced random data. It uses a
BLAKE2s hash function to hash the generated random data and then signs
the hash digest using Ed25519 with the private key.

`tkey-random-generator` will then print out the random data, the
public key, the signature, and the hash digest of the data. It will
compute the hash digest over the random data to verify it's the same
and then attempt to verify the signature.

Use the `verify` command if you want to verify the signature on the
random data at a later time. See below.

## Usage

```
tkey-random-generator <command> [flags] FILE...
```
where the commands are
```
  generate    Generate random data
  verify      Verify signature of previously generated data

  Flags:
      --version   Output version information.
```

Usage for `generate` command

```
tkey-random-generator generate <bytes> [-s] [--uss] [flags..]
```
with the flags

```
  -p, --port PATH       Set serial port device PATH. If this is not
                        passed, auto-detection will be attempted.
      --speed BPS       Set serial port speed in BPS (bits per second).
                        (default 62500)
  -s, --signature       Get the signature of the generated random data.
  -f, --file FILE       Output random data as binary to FILE.
  -h, --help            Output this help.
      --uss             Enable typing of a phrase to be hashed as the User
                        Supplied Secret. The USS is loaded onto the TKey
                        along with the app itself. A different USS results
                        in different Compound Device Identifier, different
                        start of the random sequence, and another key pair
                        used for signing.
      --uss-file FILE   Read FILE and hash its contents as the USS. Use
                        '-' (dash) to read from stdin. The full contents
                        are hashed unmodified (e.g. newlines are not stripped).
  -v, --verbose         Be more verbose
```

Usage for `verify` command
```
tkey-random-generator verify FILE SIG-FILE PUBKEY-FILE [-b]
```
with flags
```
  -b, --binary   Specify if the input FILE is in binary format.
  -h, --help     Output this help.
```

i.e. run

```
$ tkey-random-generator generate 256 -s
```
in order to generate 256 bytes of signed random data.

Store the random data, the signature and the public key printed and run

```
$ tkey-random-generator verify random_data_file signature_file public_key_file
```
in order to verify previously generated data.

Please see the [Developer
Handbook](https://dev.tillitis.se/tools/#qemu) for [how to run with
QEMU](https://dev.tillitis.se/tools/#qemu).


## Application protocol

The `Random generator` has a simple protocol on top of the [TKey
Framing Protocol](https://dev.tillitis.se/protocol/#framing-protocol)
with the following requests:

| *command*             | *FP length* | *code* | *data*                              | *response*            |
|-----------------------|-------------|--------|-------------------------------------|-----------------------|
| `CMD_GET_NAMEVERSION` | 1 B         | 0x01   | none                                | `RSP_GET_NAMEVERSION` |
| `CMD_GET_RANDOM`      | 4 B         | 0x03   | Number of bytes, 1 < x < 126        | `RSP_GET_RANDOM`      |
| `CMD_GET_PUBKEY`      | 1 B         | 0x05   | none                                | `RSP_GET_PUBKEY`      |
| `CMD_GET_SIG`         | 1 B         | 0x07   | none                                | `RSP_GET_SIG`         |


| *response*            | *FP length* | *code* | *data*                              |
|-----------------------|-------------|--------|-------------------------------------|
| `RSP_GET_NAMEVERSION` | 32 B        | 0x02   | 2 * 4 bytes name, version 32 bit LE |
| `RSP_GET_RANDOM`      | 128 B       | 0x04   | Up to 126 bytes of random data      |
| `RSP_GET_PUBKEY`      | 128 B       | 0x06   | 32 bytes Ed25519 public key         |
| `RSP_GET_SIG`         | 128 B       | 0x08   | 64B Ed25519 signature + 32B hash    |
| `RSP_UNKNOWN_CMD`     | 1 B         | 0xff   | none                                |

| *status replies* | *code* |
|------------------|--------|
| OK               | 0      |
| BAD              | 1      |

It identifies itself with:

- `name0`: "tk1  "
- `name1`: "rand"

Please note that application replies with a `NOK` Framing Protocol
response status if the endpoint field in the FP header is meant for
the firmware (endpoint = `DST_FW`). This is recommended for
well-behaved device applications so the client side can probe for the
firmware.

Typical use by a client application:

1. Probe for firmware by sending firmware's `GET_NAME_VERSION` with
   FPheader endpoint = `DST_FW`.
2. If firmware is found, load the device application.
3. Upon receiving the device app digest back from firmware, switch to
   start talking the protocol depicted above.
4. Send `CMD_GET_RANDOM` with wanted number of bytes as argument to
   generated and retrieve random data.
5. Repeat step 4 until wanted amount of random data is received.
6. Send `CMD_GET_SIG` to calculate and get the signature and hash.
8. Send `CMD_GET_PUBKEY` to receive the public key. If the public
   key is already stored, check against it so it's the expected TKey.

**Please note**: `CMD_GET_SIG` should always be sent after the last
random data response is received. This is in order to prevent old hash
data to be include if one fetches more random data without resetting
the TKey. This is done automatically in `tkey-random-generator`.

**Please note**: The firmware detection mechanism is not by any means
secure. If in doubt a user should always remove the TKey and insert it
again before doing any operation.

## Embedded device app

The `random-generator` device app embedded into
`tkey-random-generator` is built from
https://github.com/tillitis/tkey-random-generator tag v0.0.2.

## Building & installing

### Default `tkey-random-generator`

Install with:

```
go install github.com/tillitis/tkey-random-generator/cmd/tkey-random-generator@latest
```

Building locally:

```
make
```

**NOTE WELL**: This gives you the pre-compiled `random-generator`
device app embedded in the client app. See below if you want the
development version of the device app.

See also `make menu` for a list of targets.

### Building during development

You have two options for build tools: either use our OCI image
`ghcr.io/tillitis/tkey-builder` or native tools.

An easy way to build is to use the provided scripts:

- `build.sh` for native tools.
- `build-podman.sh` for use with Podman.

These scripts automatically clone the [tkey-libs device
libraries](https://github.com/tillitis/tkey-libs) in a directory next
to this one.

If you want to compile manually, see the `make menu`.

### Make variables

- `LIBDIR`: If you cloned `tkey-libs` to somewhere else then the
   default, set to the path of the directory.
- `OBJCOPY`: If your `objcopy` is called something else than
  `llvm-objcopy`
  
Please see [Tools & libraries](https://dev.tillitis.se/tools/) in the Development Handbook
for detailed information on the currently supported build and development environment.

### Building with another `random-generator`

For convenience, and to be able to support `go install`, a precompiled
`random-generator` binary is included under
`cmd/tkey-random-generator/`.

If you want to change the included device app, choose to use the
development target documented above under [Building during
development](#building-during-development) or, if you want to change
it more permanently for a release:

1. Compile your own `random-generator` and place it in the
   `cmd/tkey-random-generator/` directory with a descriptive name,
   something like `random-generator.bin-v0.0.2`.
2. Change the path to the embedded binary in
   `cmd/tkey-random-generator/appbinary.go`. Look for `go:embed...`.
4. Compute a new SHA-512 hash digest for your binary, typically by
   something like `sha512sum .bin-${signer_version}` and put the
   resulting output in a file next to the binary with the suffix
   `.sha512`.
5. Add the digest file to the `make-check` make target.

## Licenses and SPDX tags

Unless otherwise noted, the project sources are licensed under the
terms and conditions of the "GNU General Public License v2.0 only":

> Copyright Tillitis AB.
>
> These programs are free software: you can redistribute it and/or
> modify it under the terms of the GNU General Public License as
> published by the Free Software Foundation, version 2 only.
>
> These programs are distributed in the hope that they will be useful,
> but WITHOUT ANY WARRANTY; without even the implied warranty of
> MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
> General Public License for more details.

> You should have received a copy of the GNU General Public License
> along with this program. If not, see:
>
> https://www.gnu.org/licenses

See [LICENSE](LICENSE) for the full GPLv2-only license text.

External source code we have imported are isolated in their own
directories. They may be released under other licenses. This is noted
with a similar `LICENSE` file in every directory containing imported
sources.

The project uses single-line references to Unique License Identifiers
as defined by the Linux Foundation's [SPDX project](https://spdx.org/)
on its own source files, but not necessarily imported files. The line
in each individual source file identifies the license applicable to
that file.

The current set of valid, predefined SPDX identifiers can be found on
the SPDX License List at:

https://spdx.org/licenses/
