# TKey Random Generator

The TKey `Random Generator` application is a hardware-based
source of high-quality random numbers. The generator is using a
Hash_DRBG built around the BLAKE2s hash function. The application can
also sign the generated random data in order to provide proof of its
origin.

## Signed Random Data
The `Random Generator` can sign the produced random data. It uses a
BLAKE2s hash function to hash the generated random data, which it
signs using the private key and Ed25519.

One can retrieve the hash, public key and signature in order to
validate the generated data. This functionality is implemented in
the provided client application and can be used as an example.

## Client application
A client application, `tkey-random-generator`, is provided in Go. It
embeds the device application and automatically loads it onto the
TKey if needed.

### Usage
```
  -p, --port PATH     Set serial port device PATH. If this is not passed,
                      auto-detection will be attempted.
      --speed BPS     Set serial port speed in BPS (bits per second).
                      (default 62500)
  -b, --bytes COUNT   Fetch COUNT number of random bytes.
  -s, --signature     Get the signature of the generated random data.
  -f, --file FILE     Output random data as binary to FILE.
  -h, --help          Output this help.
  ```

i.e. run

```
$ ./tkey-random-generator -b 256 -s
```
in order to generate 256 bytes of signed random data.

Please see the [Developer
Handbook](https://dev.tillitis.se/tools/#qemu) for [how to run with
QEMU](https://dev.tillitis.se/tools/#qemu).


## Application protocol

`Random generator` has a simple protocol on top of the [TKey Framing
Protocol](https://dev.tillitis.se/protocol/#framing-protocol) with the
following requests:

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
5. Repeat step 4 until wanted amount of random data is recieved.
6. Send `CMD_GET_SIG` to calculate and get the signature and hash.
8. Send `CMD_GET_PUBKEY` to receive the public key. If the public
   key is already stored, check against it so it's the expected TKey.

**Please note**: `CMD_GET_SIG` should always be sent after the last
random data response is recieved. This is in order to prevent old
hash data to be include if one fetches more random data without
reseting the TKey. This is done automatically in the client-app.

**Please note**: The firmware detection mechanism is not by any means
secure. If in doubt a user should always remove the TKey and insert it
again before doing any operation.

## Building

You have two options for build tools: either use our OCI image
`ghcr.io/tillitis/tkey-builder` or native tools.

In either case, you need the device libraries in a directory next to
this one. The device libraries are available in:

https://github.com/tillitis/tkey-libs

Clone and build the device libraries first. You will most likely want
to specify a release with something like `-b v0.0.1`:

```
$ git clone -b v0.0.1 --depth 1 https://github.com/tillitis/tkey-libs
$ cd tkey-libs
$ make
```

### Building with Podman

We provide an OCI image with all the tools needed to build both `tkey-libs`
and this application. If you have `make` and Podman installed you can use it
for the `tkey-libs` directory and then this directory:

```
make podman
```

and everything should be built. This assumes a working rootless
podman. On Ubuntu 22.10, running

```
apt install podman rootlesskit slirp4netns
```

should be enough to get you a working Podman setup.

### Building with native tools

To build with native tools you need at least the `clang`, `llvm`,
`lld`, packages installed. Version 15 or later of LLVM/Clang is for
support of our architecture (RV32\_Zmmul). Ubuntu 22.10 (Kinetic) is
known to have this. Please see
[toolchain_setup.md](https://github.com/tillitis/tillitis-key1/blob/main/doc/toolchain_setup.md)
(in the tillitis-key1 repository) for detailed information on the
currently supported build and development environment.

Build everything:

```
$ make
```

If you cloned `tkey-libs` to somewhere else then the default, set
`LIBDIR` to the path of the directory.

If your available `objcopy` is anything other than the default
`llvm-objcopy`, then define `OBJCOPY` to whatever they're called on
your system.

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
