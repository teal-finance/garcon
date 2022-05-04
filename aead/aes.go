// #region <editor-fold desc="Preamble">
// Copyright (c) 2022 Teal.Finance contributors
//
// This file is part of Teal.Finance/Garcon, an API and website server.
// Teal.Finance/Garcon is free software: you can redistribute it
// and/or modify it under the terms of the GNU Lesser General Public License
// either version 3 or any later version, at the licensee’s option.
// SPDX-License-Identifier: LGPL-3.0-or-later
//
// Teal.Finance/Garcon is distributed WITHOUT ANY WARRANTY.
// For more details, see the LICENSE file (alongside the source files)
// or online at <https://www.gnu.org/licenses/lgpl-3.0.html>
// #endregion </editor-fold>

// package aead provides Encrypt() and Decrypt() for
// AEAD (Authenticated Encryption with Associated Data).
// see https://wikiless.org/wiki/Authenticated_encryption
//
// This package has been inspired from:
// - https://go.dev/blog/tls-cipher-suites
// - https://github.com/gtank/cryptopasta
//
// The underlying algorithm is AES-128 GCM:
// - AES is a symmetric encryption, faster than asymmetric (e.g. RSA)
// - 128-bit key is sufficient for most usages (256-bits is much slower)
//
// Assumption design: This library should be used on AES-supported hardware
// like AMD/Intel processors providing optimized AES instructions set.
// If this is not your case, please repport a feature request
// to implement support for ChaCha20Poly1305.
//
// GCM (Galois Counter Mode) is preferred over CBC (Cipher Block Chaining)
// because of CBC-specific attacks and configuration difficulties.
// But, CBC is faster and does not have any weakness in our server-side use case.
// If requested, this implementation may change to use CBC.
// Your feedback or suggestions are welcome, please contact us.
//
// This package follows the Golang Cryptography Principles:
// https://golang.org/design/cryptography-principles
// Secure implementation, faultlessly configurable,
// performant and state-of-the-art updated.
package aead

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"log"
)

type Cipher struct {
	gcm   cipher.AEAD
	nonce []byte
}

// prefer 16 bytes (AES-128, faster) over 32 (AES-256, irrelevant extra security)
func New(secretKey [16]byte) (c Cipher, err error) {
	block, err := aes.NewCipher(secretKey[:])
	if err != nil {
		return c, err
	}

	c.gcm, err = cipher.NewGCM(block)
	if err != nil {
		return c, err
	}

	// Never use more than 2^32 random nonces with a given key
	// because of the risk of a repeat (birthday attack).
	c.nonce = make([]byte, c.gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, c.nonce)

	return c, err
}

// Encrypt encrypts data using 256-bit AES-GCM.  This both hides the content of
// the data and provides a check that it hasn't been altered. Output takes the
// form nonce|ciphertext|tag where '|' indicates concatenation.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	ciphertext := c.gcm.Seal(nil, c.nonce, plaintext, nil)

	log.Printf("Encrypt: %d %x\n", len(ciphertext), ciphertext)

	return ciphertext, nil
}

// Decrypt decrypts data using 256-bit AES-GCM.  This both hides the content of
// the data and provides a check that it hasn't been altered. Expects input
// form nonce|ciphertext|tag where '|' indicates concatenation.
func (c *Cipher) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	plaintext, err = c.gcm.Open(nil, c.nonce, ciphertext, nil)

	log.Printf("Decrypt: %d %x\n", len(plaintext), plaintext)

	return plaintext, err
}
