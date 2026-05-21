// Copyright (C) 2022, 2023 - Tillitis AB
// SPDX-License-Identifier: GPL-2.0-only

#include <stddef.h>
#include <string.h>
#include "app_proto.h"

// Send reply frame with response status Not OK (NOK==1), shortest length
void appreply_nok(struct frame_header hdr)
{
	uint8_t buf = 0;
	frame_write(FRAME_STATUS_NOK, hdr.id, hdr.f_domain, &buf, sizeof(buf));
}

// Send app reply with frame header, response code, and LEN_X-1 bytes from buf
void appreply(struct frame_header hdr, enum appcmd rspcode, void *buf)
{
	size_t nbytes;
	// enum cmdlen len;

	uint8_t frame[128]; // longest response

	switch (rspcode) {
	case APP_RSP_GET_NAMEVERSION:
		// len = LEN_32;
		nbytes = 32;
		break;

	case APP_RSP_GET_RANDOM:
	case APP_RSP_GET_ENTROPY:
		// len = LEN_128;
		nbytes = 128;
		break;

	case APP_RSP_GET_PUBKEY:
		// len = LEN_128;
		nbytes = 128;
		break;

	case APP_RSP_GET_SIG:
		// len = LEN_128;
		nbytes = 128;
		break;

	default:
		// qemu_puts("appreply(): Unknown response code: ");
		// qemu_puthex(rspcode);
		// qemu_lf();
		//
		return;
	}
	frame[0] = rspcode;
	memcpy(&frame[1], buf, nbytes - 1);
	frame_write(FRAME_STATUS_OK, hdr.id, hdr.f_domain, frame, nbytes);

	// // Frame Protocol Header
	// writebyte(genhdr(hdr.id, hdr.endpoint, 0x0, len));
	//
	// // app protocol header is 1 byte response code
	// writebyte(rspcode);
	// nbytes--;
	//
	// write(buf, nbytes);
}
