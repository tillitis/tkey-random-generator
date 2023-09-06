// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#include <tkey/blake2s.h>
#include <tkey/lib.h>
#include <tkey/qemu_debug.h>
#include <tkey/tk1_mem.h>

#include "rng.h"

// clang-format off
static volatile	uint32_t *cdi =          (volatile uint32_t *)TK1_MMIO_TK1_CDI_FIRST;
static volatile uint32_t *trng_status  = (volatile uint32_t *)TK1_MMIO_TRNG_STATUS;
static volatile uint32_t *trng_entropy = (volatile uint32_t *)TK1_MMIO_TRNG_ENTROPY;

#define RESEED_TIME 1000
// clang-format on

uint8_t rng_initalized = 0;

static uint32_t entropy_get()
{
	while ((*trng_status & (1 << TK1_MMIO_TRNG_STATUS_READY_BIT)) == 0) {
	}
	return *trng_entropy;
}

static void rng_update(rng_ctx *ctx)
{
	for (int i = 0; i < 8; i++) {
		ctx->state[i] = ctx->digest[i];
	}

	ctx->state_ctr_lsb += 1;
	if (ctx->state_ctr_lsb == 0) {
		ctx->state_ctr_msb += 1;
	}
	ctx->state[14] += ctx->state_ctr_msb;
	ctx->state[15] += ctx->state_ctr_lsb;

	ctx->reseed_ctr += 1;
	if (ctx->reseed_ctr == RESEED_TIME) {
		for (int i = 0; i < 8; i++) {
			ctx->state[i + 8] = entropy_get();
		}
		ctx->reseed_ctr = 0;
	}
}

void rng_init(rng_ctx *ctx)
{
	qemu_puts("Init rng state\n");

	for (int i = 0; i < 8; i++) {
		ctx->state[i] = cdi[i];
		ctx->state[i + 8] = entropy_get();
	}

	ctx->state_ctr_lsb = entropy_get();
	ctx->state_ctr_msb = entropy_get();

	ctx->reseed_ctr = 0;

	// Perform initial mixing of state.
	blake2s_ctx b2s_ctx;
	blake2s(ctx->digest, 32, NULL, 0, ctx->state, 64, &b2s_ctx);
	rng_update(ctx);

	rng_initalized = 1;
}

int rng_get(uint32_t *output, rng_ctx *ctx, int size)
{
	if (size < 1 || rng_initalized == 0) {
		return -1;
	}

	blake2s_ctx b2s_ctx;
	int left = size;

	qemu_puts("nbr bytes: ");
	qemu_putinthex((uint32_t)size);
	qemu_lf();

	int i = 0;
	int gen_size = 16; // max output in one round
	while (left > 0) {

		blake2s(ctx->digest, 32, NULL, 0, ctx->state, 64, &b2s_ctx);
		memcpy(&output[i], ctx->digest, gen_size);
		rng_update(ctx);
		left -= gen_size;
		i += 4;
	}
	qemu_puts("get rand out: \n");
	qemu_hexdump((uint8_t *)output, size);
	return 0;
}
