# Release notes

## v0.0.3

- Update tkeyclient version because of a vulnerability leaving some
  USSs unused. Keys might have changed since earlier versions! Read
  more here:

  https://github.com/tillitis/tkeyclient/security/advisories/GHSA-4w7r-3222-8h6v

  The error is only triggered if you use `tkey-random-generator` with
  the `--uss` or `--uss-file` flags and an affected USS. An affected
  USS hashes to a digest with a 0 (zero) in the first byte.

  Follow these steps to identify if you are affected:

  1. Run `tkey-random-generator generate -s --uss 1` which forces the
     use of a USS.
  2. Type in your USS.
  3. Note the public key that is printed.
  4. Remove and reinsert the TKey.
  5. Run `tkey-random-generator generate -s 1`, signing without a USS.
  6. Compare the public key output from both commands. If they are the
     same your USS is vulnerable.

  If your USS are affected, you have three options:

  1. Not using a USS and keep your signing keys.
  2. Keep using the USS and get new signing keys.
  3. Use another USS and get new signing keys.

- Add a new option flag: `--force-full-uss` to force full use of the
  32 byte USS digest.

[Complete changelog](https://github.com/tillitis/tkey-random-generator/compare/v0.0.2...v0.0.3).

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
