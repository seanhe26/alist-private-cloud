package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/alist-org/alist/v3/internal/errs"
	"hash"
	"io"
	"strings"
)

func GetSHA1Encode(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GetSHA256Encode(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GetMD5Encode(data []byte) string {
	h := md5.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func GetMD5EncodeStr(data string) string {
	return GetMD5Encode([]byte(data))
}

//inspired by "github.com/rclone/rclone/fs/hash"

// ErrUnsupported should be returned by filesystem,
// if it is requested to deliver an unsupported hash type.
var ErrUnsupported = errors.New("hash type not supported")

// Type indicates a standard hashing algorithm
type hashType struct {
	Width   int
	Name    string
	Alias   string
	NewFunc func() hash.Hash
}

var (
	name2hash  = map[string]*hashType{}
	alias2hash = map[string]*hashType{}
	Supported  []*hashType
)

// RegisterHash adds a new Hash to the list and returns it Type
func RegisterHash(name, alias string, width int, newFunc func() hash.Hash) *hashType {

	newType := &hashType{
		Name:    name,
		Alias:   alias,
		Width:   width,
		NewFunc: newFunc,
	}

	name2hash[name] = newType
	alias2hash[alias] = newType
	Supported = append(Supported, newType)
	return newType
}

var (
	// MD5 indicates MD5 support
	MD5 = RegisterHash("md5", "MD5", 32, md5.New)

	// SHA1 indicates SHA-1 support
	SHA1 = RegisterHash("sha1", "SHA-1", 40, sha1.New)

	// SHA256 indicates SHA-256 support
	SHA256 = RegisterHash("sha256", "SHA-256", 64, sha256.New)
)

// HashData get hash of one hashType
func HashData(hashType *hashType, data []byte) string {
	h := hashType.NewFunc()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// HashReader get hash of one hashType from a reader
func HashReader(hashType *hashType, reader io.Reader) (string, error) {
	h := hashType.NewFunc()
	_, err := io.Copy(h, reader)
	if err != nil {
		return "", errs.NewErr(err, "HashReader error")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fromTypes will return hashers for all the requested types.
func fromTypes(types []*hashType) map[*hashType]hash.Hash {
	hashers := map[*hashType]hash.Hash{}
	for _, t := range types {
		hashers[t] = t.NewFunc()
	}
	return hashers
}

// toMultiWriter will return a set of hashers into a
// single multiwriter, where one write will update all
// the hashers.
func toMultiWriter(h map[*hashType]hash.Hash) io.Writer {
	// Convert to to slice
	var w = make([]io.Writer, 0, len(h))
	for _, v := range h {
		w = append(w, v)
	}
	return io.MultiWriter(w...)
}

// A MultiHasher will construct various hashes on all incoming writes.
type MultiHasher struct {
	w    io.Writer
	size int64
	h    map[*hashType]hash.Hash // Hashes
}

// NewMultiHasher will return a hash writer that will write
// the requested hash types.
func NewMultiHasher(types []*hashType) *MultiHasher {
	hashers := fromTypes(types)
	m := MultiHasher{h: hashers, w: toMultiWriter(hashers)}
	return &m
}

func (m *MultiHasher) Write(p []byte) (n int, err error) {
	n, err = m.w.Write(p)
	m.size += int64(n)
	return n, err
}

func (m *MultiHasher) GetHashInfo() *HashInfo {
	dst := make(map[*hashType]string)
	for k, v := range m.h {
		dst[k] = hex.EncodeToString(v.Sum(nil))
	}
	return &HashInfo{h: dst}
}

// Sum returns the specified hash from the multihasher
func (m *MultiHasher) Sum(hashType *hashType) ([]byte, error) {
	h, ok := m.h[hashType]
	if !ok {
		return nil, ErrUnsupported
	}
	return h.Sum(nil), nil
}

// Size returns the number of bytes written
func (m *MultiHasher) Size() int64 {
	return m.size
}

// A HashInfo contains hash string for one or more hashType
type HashInfo struct {
	h map[*hashType]string
}

func NewHashInfo(ht *hashType, str string) HashInfo {
	m := make(map[*hashType]string)
	m[ht] = str
	return HashInfo{h: m}
}

func (hi HashInfo) String() string {
	var tmp []string
	for ht, str := range hi.h {
		if len(str) > 0 {
			tmp = append(tmp, ht.Name+":"+str)
		}
	}
	return strings.Join(tmp, "\n")
}
func (hi HashInfo) GetHash(ht *hashType) string {
	return hi.h[ht]
}
