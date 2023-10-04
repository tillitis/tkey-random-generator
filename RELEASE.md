# Release notes

## v0.0.2

This version has multiple improvements and new features. One is the
possiblity to verify a signature over a random data set.

Security wise the initial random state before random generation is no
longer just the CDI. The state is instead first the CDI with
concatenated entropy and then immediately mixed away by an initial
round of BLAKE2s. This is done before any generation and output of
random data even takes place to remove even the slightest chance of
leaking any parts of the CDI.

We also use an additional counter to prevent a loop of generated
random data if the TRNG for some reason would stop working.

**Features:**
- Verify a signatures over a random data set, making it possible to
  show proof of the origin of data.
- You can now enter a User Supplied Secret (USS) which makes it
  possible to affect the CDI and the signing identity with user input.


**Improvements:**
- Now has two subcommands, "generate" and "verify".
- Added `--verbose` flag to show progress when generating large amount
  of data to a binary file.
- Seperated the state cnt in rng to two separate counters. One to
  evolve the state and one to keep track of when reseed should be
  performed.
- Concatenate entropy to the CDI and perform an rng update as part of
  initialisation.
- Added a man page.
- Use latest TKey-libs v0.0.2.
- Updated to use Monocypher 4 for Ed25519 signatures (part of
  tkey-libs).
- Added `--version` flag.

**Bug fixes:**
- Wrong size of the digest in the rng context.
- Updated Go serial package (from tkeyclient) to make the app
  buildable on Darwin/macOS for Go 1.21.

[Complete changelog](https://github.com/tillitis/tkey-random-generator/compare/v0.0.1...v0.0.2).


## v0.0.1

First relese of the tkey-random-generator.

Makes it possible to generate large amount of high quality random
data, with the possibility to sign it as a proof of origin.
