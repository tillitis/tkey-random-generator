tkey(1)

# NAME

tkey-random-generator - A hardware-backed random number generator

# SYNOPSIS

*tkey-random-generator* generate BYTES [-s] [--uss] [options...]

*tkey-random-generator* verify FILE SIG-FILE PUBKEY-FILE [-b] [options...]

# DESCRIPTION

*tkey-random-generator* is a hardware-backed source of high-quality
random numbers for use with the Tillitis TKey.

# OPTIONS

Common options for both commands:

*-h, --help*

	Output help text.

# COMMANDS

## generate

*tkey-random-generator* generate BYTES [-s] [--uss] [common options...]

Generates BYTES bytes of random data, optionally signed with Ed25519
to provide proof of origin.

Output can be chosen between stdout (in hexadecimal) or a binary file.

*-f, --file FILE*

	Output random data as binary to FILE.

*-p*, *--port PATH*

	Set serial port device PATH. If this is not passed, auto-detection
	will be attempted.


*-s, --signature*

	Request an Ed25519 signature of the random data.

*--speed BPS*

	Set serial port speed to BPS b/s. Default is 62500 b/s.

*--uss*

	Ask for a phrase to be hashed as the User Supplied Secret. The
	USS is loaded onto the TKey along with the app itself. A
	different USS results in different Compound Device Identifier,
	a different initialisation of the random sequence, and another
	key pair used for signing.

*--uss-file FILE*

	Read FILE and hash its contents as the USS. Use '-' (dash) to read
	from stdin. The full contents are hashed unmodified (e.g. newlines are not stripped).

*--verbose*

	Be more verbose, including reporting progress when writing to
	a file.

## verify

*tkey-random-generator* verify FILE SIG-FILE PUBKEY-FILE [-b] [common
options...]

Verifies the Ed25519 signature of FILE. Does not need a connected TKey
to verify.

FILE is assumed to be a hexadecimal representation of the random data
from the *generate* command. Use *-b* if binary. SIG-FILE is expected
to be 64 bytes Ed25519 signature in hex. PUBKEY-FILE is expected to be
32 bytes Ed25519 public key in hex.

The exit code is 0 if the signature is valid, otherwise non-zero.
Newlines will be stripped from the input files.

Options:

*-b, --binary*

	Specify if the input FILE is in binary format.

*-h, --help*

	Output this help.

# EXAMPLES

Generate signed random data in a binary file and ask for a User
Supplied Secret:

```
./tkey-random-generator generate 4711 -s --uss -f random.dat
Auto-detected serial port /dev/ttyACM0
Connecting to device on serial port /dev/ttyACM0...
Enter phrase for the USS:
Repeat the phrase:
Writing 4711 bytes of random data to: random.dat


Public key: 329f0d5c806409508d359bd562fe7e5963b8c7e9b767a22681d55e7783736170
Signature: c98719eb80ed3f20e0ade71a75d21c90d0a87760aa78bc53d507c1a4a42cccae8986e4aa215ca1936939b1016fd7c12c854056bacefc4e2e841e598a2e693303
Hash: 6fcd14991295926d81dff0c1e77e2da6325938dad3a06ee79cc19a4bc89a0f26

Verifying signature ...
signature verified.
```

To verify this signature later, store the public key and the signature
in files, let's say *pk* and *sig*. Then run:

```
./tkey-random-generator verify -b random.dat sig pk
Verifying signature ...
Public key: 329f0d5c806409508d359bd562fe7e5963b8c7e9b767a22681d55e7783736170
Signature: c98719eb80ed3f20e0ade71a75d21c90d0a87760aa78bc53d507c1a4a42cccae8986e4aa215ca1936939b1016fd7c12c854056bacefc4e2e841e598a2e693303
BLAKE2s hash: 6fcd14991295926d81dff0c1e77e2da6325938dad3a06ee79cc19a4bc89a0f26
Signature verified.
```

# SEE ALSO

https://tillitis.se/
