// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#include <monocypher/monocypher-ed25519.h>
#include <stdint.h>
#include <tkey/qemu_debug.h>
#include <tkey/tk1_mem.h>

#include "app_proto.h"
#include "blake2s/blake2s.h"
#include "rng.h"

// clang-format off
static volatile	uint32_t *cdi =          (volatile uint32_t *)TK1_MMIO_TK1_CDI_FIRST;
static volatile uint32_t *led =          (volatile uint32_t *)TK1_MMIO_TK1_LED;
static volatile uint32_t *cpu_mon_ctrl  = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_CTRL;
static volatile uint32_t *cpu_mon_first = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_FIRST;
static volatile uint32_t *cpu_mon_last  = (volatile uint32_t *) TK1_MMIO_TK1_CPU_MON_LAST;
static volatile uint32_t *app_addr  	= (volatile uint32_t *) TK1_MMIO_TK1_APP_ADDR;
static volatile uint32_t *app_size 	= (volatile uint32_t *) TK1_MMIO_TK1_APP_SIZE;

#define LED_BLACK  0
#define LED_RED    (1 << TK1_MMIO_TK1_LED_R_BIT)
#define LED_GREEN  (1 << TK1_MMIO_TK1_LED_G_BIT)
#define LED_BLUE   (1 << TK1_MMIO_TK1_LED_B_BIT)

// clang-format on

const uint8_t app_name0[4] = "tk1 ";
const uint8_t app_name1[4] = "rand";
const uint32_t app_version = 0x00000001;

// RSP_GET_RANDOM_cmdlen - (responsecode + status)
#define RANDOM_PAYLOAD_MAXBYTES 128 - (1 + 1)

int main(void)
{
	uint32_t stack;
	struct frame_header hdr; // Used in both directions
	uint8_t cmd[CMDLEN_MAXBYTES];
	uint8_t rsp[CMDLEN_MAXBYTES];
	uint8_t in;
	uint32_t digest[32];
	uint8_t pubkey[32];
	uint32_t local_cdi[8];
	uint8_t secret_key[64];
	uint8_t signature[64];
	uint8_t hash[32];
	uint8_t rand_data_generated = 0;
	rng_ctx rng_ctx;
	blake2s_ctx b2s_ctx;

	// Use Execution Monitor on RAM after app
	*cpu_mon_first = *app_addr + *app_size;
	*cpu_mon_last = TK1_RAM_BASE + TK1_RAM_SIZE;
	*cpu_mon_ctrl = 1;

	qemu_puts("Hello, I'm randomapp! &stack is on: ");
	qemu_putinthex((uint32_t)&stack);
	qemu_lf();

	// Generate public key
	wordcpy(local_cdi, (void *)cdi, 8);
	crypto_ed25519_key_pair(secret_key, pubkey, (uint8_t *)local_cdi);

	// Initialise the rng
	rng_init(&rng_ctx);

	// Init hash
	blake2s_init(&b2s_ctx, 32, NULL, 0);

	*led = LED_RED | LED_BLUE;
	for (;;) {
		in = readbyte();
		qemu_puts("Read byte: ");
		qemu_puthex(in);
		qemu_lf();

		if (parseframe(in, &hdr) == -1) {
			qemu_puts("Couldn't parse header\n");
			continue;
		}

		memset(cmd, 0, CMDLEN_MAXBYTES);
		// Read app command, blocking
		read(cmd, hdr.len);

		if (hdr.endpoint == DST_FW) {
			appreply_nok(hdr);
			qemu_puts("Responded NOK to message meant for fw\n");
			continue;
		}

		// Is it for us?
		if (hdr.endpoint != DST_SW) {
			qemu_puts("Message not meant for app. endpoint was 0x");
			qemu_puthex(hdr.endpoint);
			qemu_lf();
			continue;
		}

		// Reset response buffer
		memset(rsp, 0, CMDLEN_MAXBYTES);

		// Min length is 1 byte so this should always be here
		switch (cmd[0]) {
		case APP_CMD_GET_NAMEVERSION:
			qemu_puts("APP_CMD_GET_NAMEVERSION\n");
			// only zeroes if unexpected cmdlen bytelen
			if (hdr.len == 1) {
				memcpy(rsp, app_name0, 4);
				memcpy(rsp + 4, app_name1, 4);
				memcpy(rsp + 8, &app_version, 4);
			}
			appreply(hdr, APP_RSP_GET_NAMEVERSION, rsp);
			break;

		case APP_CMD_GET_RANDOM:
			qemu_puts("APP_CMD_GET_RANDOM\n");
			if (hdr.len != 4) {
				qemu_puts(
				    "APP_CMD_GET_RANDOM bad cmd length\n");
				break;
			}

			// cmd[1] is number of bytes requested
			uint8_t bytes = cmd[1];
			if (bytes < 1 || bytes > RANDOM_PAYLOAD_MAXBYTES) {
				qemu_puts("Requested bytes outside range\n");
				rsp[0] = STATUS_BAD;
				appreply(hdr, APP_RSP_GET_RANDOM, rsp);
				break;
			}
			rsp[0] = STATUS_OK;

			rng_get(digest, &rng_ctx, bytes);
			memcpy(rsp + 1, digest, bytes);
			appreply(hdr, APP_RSP_GET_RANDOM, rsp);

			blake2s_update(&b2s_ctx, digest, bytes);

			rand_data_generated = 1;

			break;

		case APP_CMD_GET_PUBKEY:
			qemu_puts("APP_CMD_GET_PUBKEY\n");
			memcpy(rsp, pubkey, 32);
			appreply(hdr, APP_RSP_GET_PUBKEY, rsp);
			break;

		case APP_CMD_GET_SIG:
			qemu_puts("APP_CMD_GET_SIG\n");
			if (rand_data_generated == 0) {
				rsp[0] = STATUS_BAD;
				appreply(hdr, APP_RSP_GET_SIG, rsp);
				break;
			}
			rsp[0] = STATUS_OK;

			// Finalize hash
			blake2s_final(&b2s_ctx, hash);

			// Create the Ed25519 signature of hash
			crypto_ed25519_sign(signature, secret_key, hash,
					    sizeof(hash));

			memcpy(rsp + 1, signature, 64);
			memcpy(rsp + 1 + 64, hash, 32);
			appreply(hdr, APP_RSP_GET_SIG, rsp);

			// Re-init hash for next random generation
			blake2s_init(&b2s_ctx, 32, NULL, 0);
			rand_data_generated = 0;

			break;

		default:
			qemu_puts("Received unknown command: ");
			qemu_puthex(cmd[0]);
			qemu_lf();
			appreply(hdr, APP_RSP_UNKNOWN_CMD, rsp);
		}
	}
}
