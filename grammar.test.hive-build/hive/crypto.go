package hive

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"runtime"
	"strconv"
	"strings"
	"unsafe"
)

type CryptoCryptoError struct {
	Reason  string
	Message string
}

func (e CryptoCryptoError) Error() string {
	return "hive: crypto " + e.Reason + ": " + e.Message
}

type CryptoHeader struct {
	Alg string
	Typ string
	Kid string
}

func hmacSha256Raw(secret []byte, input string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(input))
	return mac.Sum(nil)
}

func CryptoSha256(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

func CryptoSha512(input string) string {
	sum := sha512.Sum512([]byte(input))
	return hex.EncodeToString(sum[:])
}

func CryptoHmacSha256(input string, key Secret) string {
	defer runtime.KeepAlive(key)
	return hex.EncodeToString(hmacSha256Raw(key.bytes(), input))
}

func CryptoBase64Encode(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

func CryptoBase64Decode(input string) Result[string, CryptoCryptoError] {
	b, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return Err[string, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "input is not valid base64"})
	}
	return Ok[string, CryptoCryptoError](string(b))
}

func CryptoRandomHex(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func CryptoRandomSecret(n int) Result[Secret, SecretError] {
	raw, err := newSecret(max(n, 0))
	if err != nil {
		return Err[Secret, SecretError](*err)
	}
	defer runtime.KeepAlive(raw)
	rand.Read(raw.bytes())
	s, err := newSecret(2 * len(raw.bytes()))
	if err != nil {
		return Err[Secret, SecretError](*err)
	}
	hex.Encode(s.bytes(), raw.bytes())
	return Ok[Secret, SecretError](s)
}

const (
	encryptVersion = 1
	encryptSaltLen = 16

	encryptKeyLen = 32
)

func encryptWork(version byte) (int, bool) {
	switch version {
	case 1:
		return 600000, true
	}
	return 0, false
}

func encryptCipher(password []byte, salt []byte, work int) (cipher.AEAD, bool) {
	key, err := pbkdf2.Key(sha256.New, unsafe.String(unsafe.SliceData(password), len(password)), salt, work, encryptKeyLen)
	if err != nil {
		return nil, false
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	sealed, err := cipher.NewGCM(block)
	if err != nil {
		return nil, false
	}
	return sealed, true
}

func CryptoEncrypt(plaintext Secret, password Secret) string {
	defer runtime.KeepAlive(plaintext)
	defer runtime.KeepAlive(password)
	salt := make([]byte, encryptSaltLen)

	rand.Read(salt)
	work, _ := encryptWork(encryptVersion)
	sealer, ok := encryptCipher(password.bytes(), salt, work)
	if !ok {
		return ""
	}
	nonce := make([]byte, sealer.NonceSize())
	rand.Read(nonce)
	message := []byte{encryptVersion}
	message = append(message, salt...)
	message = append(message, nonce...)
	message = sealer.Seal(message, nonce, plaintext.bytes(), nil)
	return base64.StdEncoding.EncodeToString(message)
}

func CryptoDecrypt(ciphertext string, password Secret) Result[Secret, CryptoCryptoError] {
	defer runtime.KeepAlive(password)
	fail := func(reason, msg string) Result[Secret, CryptoCryptoError] {
		return Err[Secret, CryptoCryptoError](CryptoCryptoError{Reason: reason, Message: msg})
	}
	message, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return fail("Malformed", "input is not valid base64")
	}
	if len(message) < 1+encryptSaltLen {
		return fail("Malformed", "input is too short to be an encrypted message")
	}
	work, known := encryptWork(message[0])
	if !known {
		return fail("Malformed", "unknown encrypted message version "+strconv.Itoa(int(message[0])))
	}
	opener, ok := encryptCipher(password.bytes(), message[1:1+encryptSaltLen], work)
	if !ok {
		return fail("Malformed", "could not derive a key from the password")
	}
	rest := message[1+encryptSaltLen:]
	if len(rest) < opener.NonceSize()+opener.Overhead() {
		return fail("Malformed", "input is too short to be an encrypted message")
	}
	sealed := rest[opener.NonceSize():]
	plaintext, locked := newSecret(len(sealed) - opener.Overhead())
	if locked != nil {
		return fail(locked.Reason, locked.Message)
	}
	if _, err := opener.Open(plaintext.bytes()[:0], rest[:opener.NonceSize()], sealed, nil); err != nil {
		return fail("BadSignature", "wrong password, or the message was changed after it was encrypted")
	}
	return Ok[Secret, CryptoCryptoError](plaintext)
}

func CryptoJwtCodec(secret Secret) CodecCodec[CryptoJwtError] { return CodecCodec[CryptoJwtError]{Secret: secret} }

type CryptoJwtError struct {
	Path     string
	Expected string
	Found    string
}

func (e CryptoJwtError) Error() string {
	return "hive: cannot decode JWT at " + e.Path + ": expected " + e.Expected + ", found " + e.Found
}

func CryptoJwtSign(payloadJSON string, secret Secret) string {
	defer runtime.KeepAlive(secret)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))
	input := header + "." + payload
	sig := base64.RawURLEncoding.EncodeToString(hmacSha256Raw(secret.bytes(), input))
	return input + "." + sig
}

func jwtCheckTime(payload []byte) *CryptoCryptoError {
	var claims struct {
		Exp json.Number `json:"exp"`
		Nbf json.Number `json:"nbf"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return &CryptoCryptoError{Reason: "Malformed", Message: "payload is not a JSON object"}
	}
	now := int64(TimeNow())
	if claims.Exp != "" {
		if exp, err := claims.Exp.Int64(); err == nil && now >= exp {
			return &CryptoCryptoError{Reason: "Expired", Message: "token has expired"}
		}
	}
	if claims.Nbf != "" {
		if nbf, err := claims.Nbf.Int64(); err == nil && now < nbf {
			return &CryptoCryptoError{Reason: "NotYetValid", Message: "token is not valid yet"}
		}
	}
	return nil
}

func CryptoJwtParse[T any](token string, secret Secret, dec func(JsonValue, string) (T, *JsonJsonError)) Result[T, CryptoJwtError] {
	claims := jwtVerify(token, secret)
	if claims.IsError() {
		why := claims.Err()
		return Err[T, CryptoJwtError](CryptoJwtError{
			Path:     "$",
			Expected: "a JWT signed with this secret",
			Found:    why.Reason + ": " + why.Message,
		})
	}
	read := JsonParse(claims.Ok(), dec)
	if read.IsError() {
		why := read.Err()
		return Err[T, CryptoJwtError](CryptoJwtError{Path: why.Path, Expected: why.Expected, Found: why.Found})
	}
	return Ok[T, CryptoJwtError](read.Ok())
}

func jwtVerify(token string, secret Secret) Result[string, CryptoCryptoError] {
	defer runtime.KeepAlive(secret)
	fail := func(reason, msg string) Result[string, CryptoCryptoError] {
		return Err[string, CryptoCryptoError](CryptoCryptoError{Reason: reason, Message: msg})
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fail("Malformed", "expected three dot-separated segments")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return fail("Malformed", "header is not valid base64url")
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &hdr); err != nil {
		return fail("Malformed", "header is not valid JSON")
	}
	if hdr.Alg != "HS256" {
		return fail("AlgorithmMismatch", "unsupported algorithm "+strconv.Quote(hdr.Alg)+"; only HS256 is supported")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return fail("Malformed", "signature is not valid base64url")
	}

	if !hmac.Equal(sig, hmacSha256Raw(secret.bytes(), parts[0]+"."+parts[1])) {
		return fail("BadSignature", "signature does not match")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fail("Malformed", "payload is not valid base64url")
	}
	if jerr := jwtCheckTime(payload); jerr != nil {
		return Err[string, CryptoCryptoError](*jerr)
	}
	return Ok[string, CryptoCryptoError](string(payload))
}

func CryptoJwtDecode(token string) Result[string, CryptoCryptoError] {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return Err[string, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "expected at least two dot-separated segments"})
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Err[string, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "payload is not valid base64url"})
	}
	return Ok[string, CryptoCryptoError](string(payload))
}

func CryptoJwtHeader(token string) Result[CryptoHeader, CryptoCryptoError] {
	parts := strings.Split(token, ".")
	if len(parts) < 1 || parts[0] == "" {
		return Err[CryptoHeader, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "missing header segment"})
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Err[CryptoHeader, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "header is not valid base64url"})
	}
	var h struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &h); err != nil {
		return Err[CryptoHeader, CryptoCryptoError](CryptoCryptoError{Reason: "Malformed", Message: "header is not valid JSON"})
	}
	return Ok[CryptoHeader, CryptoCryptoError](CryptoHeader{Alg: h.Alg, Typ: h.Typ, Kid: h.Kid})
}
