// SPDX-FileCopyrightText: 2024 Tillitis AB <tillitis.se>
// SPDX-License-Identifier: Apache-2.0 OR MIT

#include <stddef.h>
#include <stdint.h>
#include <tkey/lib.h>
#include <string.h>
#include <tkey/tk1_mem.h>
#include "sha256/sha256.h"
#include "rng.h"

// clang-format off
static volatile	uint32_t *cdi =          (volatile uint32_t *)TK1_MMIO_TK1_CDI_FIRST;
static volatile uint32_t *trng_status =  (volatile uint32_t *)TK1_MMIO_TRNG_STATUS;
static volatile uint32_t *trng_entropy = (volatile uint32_t *)TK1_MMIO_TRNG_ENTROPY;
// clang-format on

#define RESEED_CTR 4096
#define RNG_STALE_DATA_MS (10 * 60 * 1000) // 10 minutes

 /* put here for simplicity */
void crypto_sha256(uint8_t *digest, const uint8_t *data, size_t len)
{
	SHA256_CTX ctx;
	sha256_init(&ctx);
	sha256_update(&ctx, data, len);
	sha256_final(&ctx, digest);
}

/*
 * A 440 bit (55 bytes) effective state, consisting of (in this order)
 * - 128 bit input from the last generated state, (V)
 * - 64 bit counter, incrementing each update (V)
 * - 248 bit entropi from the seeding (C)
 *
 * It is 440 bits (55 bytes) that is used to create the rng output, to prevent
 * two sha256_transform. Last byte in the last word is discarded.
 *
 * reseed_ctr increments and triggers a reseed after RESEED_CTR
 * last_updated_ms is used to assess if the data is considered stale
 *
 */
typedef struct {
	uint32_t state[14];
	uint32_t last_updated_ms;
	uint16_t reseed_ctr;
} rng_ctx;

static rng_ctx ctx;

static uint32_t entropy_get()
{
	while ((*trng_status & (1 << TK1_MMIO_TRNG_STATUS_READY_BIT)) == 0) {
	}
	return *trng_entropy;
}

/* Generates a new seed
 * Input is of length 8 words
 * Seed is an output of length 14 words
 *
 * seed = seed1[256 bits] | seed2[192 bits]
 * seed1 = sha256(input[256 bits]) | entropy[256 bits])
 * seed2 = sha256(entropy[512 bits])
 */
static void generate_seed(uint32_t *input, uint32_t *seed)
{
	uint32_t seed1[8] = {0x00};
	uint32_t seed2[8] = {0x00};
	uint32_t seed1_input[16] = {0x00};
	uint32_t seed2_input[14] = {0x00};

	for (size_t i = 0; i < 8; i++) {
		seed1_input[i] = input[i];
		seed1_input[i + 8] = entropy_get();
	}
	for (size_t i = 0; i < 14; i++) {
		seed2_input[i] = entropy_get();
	}

	crypto_sha256((uint8_t *)seed1, (uint8_t *)seed1_input, 64);
	crypto_sha256((uint8_t *)seed2, (uint8_t *)seed2_input, 55);

	memcpy((uint8_t *)seed, seed1, sizeof(seed1)); // store 256 bits
	memcpy((uint8_t *)seed + 32, seed2,
	       sizeof(seed2) - 8); // store 192 bits
	// secure_wipe(seed1, sizeof(seed1) * 4);
	// secure_wipe(seed2, sizeof(seed2) * 4);
	// secure_wipe(seed1_input, sizeof(seed1_input) * 4);
	// secure_wipe(seed2_input, sizeof(seed2_input) * 4);
}

// Input is new data that are supposed to update the state, input_len (in words)
static void rng_update_state(rng_ctx *ctx, uint32_t *input, uint8_t input_len)
{
	for (size_t i = 0; i < input_len; i++) {
		ctx->state[i] = input[i];
	}

	uint32_t *state_ctr_msb = &ctx->state[4];
	uint32_t *state_ctr_lsb = &ctx->state[5];

	*state_ctr_lsb += 1;
	if (*state_ctr_lsb == 0) {
		*state_ctr_msb += 1;
	}

	ctx->reseed_ctr += 1;
	if (ctx->reseed_ctr == RESEED_CTR) {
		// Reseed with working state as input
		generate_seed(ctx->state, ctx->state);
		ctx->reseed_ctr = 0;
	}
}

// Initialize the rng
void rng_init(void)
{
	generate_seed((uint32_t *)cdi, ctx.state);
	ctx.last_updated_ms = 1;
	// ctx.last_updated_ms = millis();
}

// Generate sz bytes of random data and put it in dst. Returns zero on
// success
int rng_get(uint8_t *dst, size_t sz)
{
	static uint32_t rng_buffer[8] = {0x00};
	static uint8_t valid_bytes = 0;

	// Check if it is initialized
	if (ctx.last_updated_ms == 0)
		return -1;

	size_t dst_index = 0;

	// uint32_t now = millis();
	uint32_t now = 2;
	if (now - ctx.last_updated_ms > RNG_STALE_DATA_MS) {
		valid_bytes = 0;
	}

	while (dst_index < sz) {
		if (valid_bytes == 0) {
			// Generate new random data, only use 440 bits as input
			// The first 16 bytes are used as output
			// the last 16 bytes are used as new_V
			crypto_sha256((uint8_t *)rng_buffer,
				      (uint8_t *)ctx.state, 55);
			rng_update_state(&ctx, rng_buffer + 4, 4);
			valid_bytes = 16;

			ctx.last_updated_ms = now;
		}

		size_t to_copy = valid_bytes;
		if (to_copy > sz - dst_index)
			to_copy = sz - dst_index;

		size_t offset = 16 - valid_bytes;

		for (size_t k = 0; k < to_copy; k++)
			dst[dst_index + k] =
			    ((uint8_t *)rng_buffer)[offset + k];

		dst_index += to_copy;
		valid_bytes -= to_copy;
	}
	return 0;
}
