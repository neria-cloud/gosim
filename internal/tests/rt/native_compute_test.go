//go:build sim

package behavior_test

import (
	"bytes"
	"crypto/aes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"hash/crc32"
	"testing"
)

func TestNativeComputation(t *testing.T) {
	sum := sha256.Sum256([]byte("abc"))
	if hex.EncodeToString(sum[:]) != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatal("SHA256 vector")
	}
	if crc32.ChecksumIEEE([]byte("123456789")) != 0xcbf43926 {
		t.Fatal("CRC32 vector")
	}
	// Standard CRC tables remain read-only, as required by hash/crc32.
	if crc32.Checksum([]byte("123456789"), crc32.MakeTable(crc32.IEEE)) != 0xcbf43926 {
		t.Fatal("CRC32 table vector")
	}
	key, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	input, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	output := make([]byte, 16)
	block.Encrypt(output, input)
	if hex.EncodeToString(output) != "69c4e0d86a7b0430d8cdb78070b4c55a" {
		t.Fatal("AES vector")
	}
	seed, _ := hex.DecodeString("9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60")
	public, private, err := ed25519.GenerateKey(bytes.NewReader(seed))
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(public) != "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a" {
		t.Fatal("Ed25519 seed vector")
	}
	signature := ed25519.Sign(private, nil)
	if !ed25519.Verify(public, nil, signature) {
		t.Fatal("Ed25519 verification")
	}
	// Pass the simulator's deterministic stream explicitly; nil uses host randomness.
	public, private, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(public, []byte("message"), ed25519.Sign(private, []byte("message"))) {
		t.Fatal("generated key")
	}
	t.Logf("generated public key %x", public)
}

func TestNamedCallbacksAndChannelMapKeys(t *testing.T) {
	type callback func(int)
	ch := make(chan int, 1)
	var send callback = func(v int) { ch <- v }
	listeners := map[chan int]string{ch: "listener"}
	go send(42)
	if <-ch != 42 || listeners[ch] != "listener" {
		t.Fatal("named callback or channel map key")
	}
}
