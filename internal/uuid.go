package internal

import (
	"crypto/rand"
	"fmt"
)

type UUID [16]byte

func (u UUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

func NewUUIDV4() UUID {
	var uuid UUID

	// Fill the entire UUID with cryptographically secure random bytes.
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(fmt.Sprintf("requestid: reading random bytes: %v", err))
	}

	// Set the version to 4.
	// The 7th byte's most significant 4 bits are set to 0100.
	// uuid[6] = (uuid[6] & 0x0F) | 0x40
	// Clear the first 4 bits and then set the 4th bit.
	uuid[6] = (uuid[6] & 0b00001111) | 0b01000000

	// Set the variant to RFC 4122 (10xx).
	// The 9th byte's most significant 2 bits are set to 10.
	// uuid[8] = (uuid[8] & 0x3F) | 0x80
	// Clear the first 2 bits and then set the 2nd bit.
	uuid[8] = (uuid[8] & 0b00111111) | 0b10000000

	return uuid
}
