---
name: cryptography-pitfalls
description: Common crypto mistakes — weak algorithms, IV reuse, padding oracle, JWT attacks, TLS misconfigs
---

# Cryptography Pitfalls

Most crypto vulnerabilities come from using the right primitives incorrectly, not from breaking the math.

## Weak Algorithms

### Never Use

```
MD5, SHA1        → collision attacks — don't use for integrity or signatures
DES, 3DES        → broken or near-broken block cipher
RC4              → biased keystream
ECB mode         → patterns preserved (penguin problem)
RSA PKCS#1 v1.5  → Bleichenbacher padding oracle
```

### Use Instead

```
Hashing:     SHA-256, SHA-3, BLAKE2
Password:    bcrypt (cost≥12), scrypt, argon2id
Symmetric:   AES-256-GCM, ChaCha20-Poly1305
Asymmetric:  RSA-OAEP or ECDH (P-256, X25519)
HMAC:        HMAC-SHA256
Key deriv:   HKDF, PBKDF2 (≥310,000 iterations for SHA-256)
```

## IV / Nonce Reuse (Fatal for GCM)

```go
// ❌ FATAL: fixed IV
iv := []byte("0000000000000000")  // never fixed
block, _ := aes.NewCipher(key)
gcm, _ := cipher.NewGCM(block)
ciphertext := gcm.Seal(nil, iv, plaintext, nil)

// ❌ Counter IV (deterministic — fine for small datasets, dangerous at scale)
iv := make([]byte, 12)
binary.BigEndian.PutUint32(iv, uint32(messageCounter))

// ✅ Random nonce (AES-GCM safe up to ~2^32 messages per key)
iv := make([]byte, gcm.NonceSize())
io.ReadFull(rand.Reader, iv)
ciphertext := gcm.Seal(iv, iv, plaintext, nil)  // prepend nonce to ciphertext
```

**GCM nonce reuse consequence:** If the same (key, nonce) pair is used twice, the attacker can XOR ciphertexts to recover the XOR of plaintexts, and forge authentication tags.

## Padding Oracle Attacks

AES-CBC with PKCS#7 padding + error distinction = padding oracle:

```
If server returns "decryption error" vs "invalid message":
→ attacker can decrypt any ciphertext byte-by-byte
→ 128-bit block = ~128 oracle queries to decrypt
```

**Fix:** Use authenticated encryption (AES-GCM, ChaCha20-Poly1305) — authentication tag prevents the oracle.

## Password Storage

```python
# ❌ Plain MD5 (cracked in seconds with rainbow tables)
hash = hashlib.md5(password.encode()).hexdigest()

# ❌ MD5 with static salt
hash = hashlib.md5((password + "SALT").encode()).hexdigest()

# ❌ SHA-256 unsalted
hash = hashlib.sha256(password.encode()).hexdigest()

# ✅ bcrypt (per-password salt built in, auto-rehash)
import bcrypt
hash = bcrypt.hashpw(password.encode(), bcrypt.gensalt(rounds=12))
valid = bcrypt.checkpw(password.encode(), hash)

# ✅ argon2id (winner of Password Hashing Competition)
from argon2 import PasswordHasher
ph = PasswordHasher(time_cost=2, memory_cost=65536, parallelism=2)
hash = ph.hash(password)
ph.verify(hash, password)
```

## JWT Attacks

### Algorithm Confusion (CVE class)

```python
# ❌ Accepts RS256 token verified with HS256 using public key as secret
# If server uses RS256, attacker can:
# 1. Get the public key
# 2. Sign a forged token with HS256 using the public key as the HMAC secret
# 3. Change alg header to HS256
# 4. Server verifies HS256 with public key → accepts forged token

# ✅ Always specify expected algorithm
jwt.decode(token, public_key, algorithms=["RS256"])  # explicit list
```

### `alg: none`

```python
# ❌
jwt.decode(token, options={"verify_signature": False})

# ✅ Never disable signature verification in production
```

### Weak HS256 Secrets

```bash
# Brute force HS256 JWT with hashcat
hashcat -a 0 -m 16500 jwt.txt wordlist.txt

# Any secret < 32 random bytes is brute-forceable
```

## TLS Misconfigurations

```go
// ❌ Disables certificate verification (MitM trivial)
&tls.Config{InsecureSkipVerify: true}

// ❌ Allows TLS 1.0/1.1 (POODLE, BEAST)
&tls.Config{MinVersion: tls.VersionTLS10}

// ❌ Weak cipher suites
// (Go's default is fine — don't specify CipherSuites unless you know what you're doing)

// ✅ Safe TLS config
&tls.Config{
    MinVersion:               tls.VersionTLS12,
    PreferServerCipherSuites: true,
    // Go 1.18+: TLS 1.3 enabled by default
}
```

## Random Number Generation

```go
// ❌ math/rand is PRNG — predictable seed
import "math/rand"
token := rand.Int63()   // NOT cryptographically random

// ✅ crypto/rand for all security-sensitive randomness
import "crypto/rand"
b := make([]byte, 32)
io.ReadFull(rand.Reader, b)
token := base64.URLEncoding.EncodeToString(b)
```

## Scanner Coverage

| Risk | Scanner | Rule ID |
|------|---------|---------|
| Weak hash (MD5/SHA1) | semgrep | `python.lang.security.audit.md5-used` |
| `InsecureSkipVerify` | gosec | G402 |
| `math/rand` for crypto | gosec | G404 |
| Hardcoded IV | semgrep | multiple |
| `alg: none` JWT | semgrep | JS/Python rules |
| Weak TLS version | semgrep, tfsec | multiple |
