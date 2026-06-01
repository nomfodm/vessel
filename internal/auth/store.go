package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "vessel-launcher"
	keyringUser    = "refresh-token"
)

// keyringStore keeps the refresh token in the OS secret store.
type keyringStore struct{}

func (keyringStore) Save(refresh string) error {
	return keyring.Set(keyringService, keyringUser, refresh)
}

func (keyringStore) Load() (string, error) {
	v, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return v, err
}

func (keyringStore) Delete() error {
	err := keyring.Delete(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// fileStore is the fallback: an AES-GCM encrypted file. The key derives from
// local machine material, so a copied file alone does not yield the token.
type fileStore struct {
	path string
	key  []byte
}

func newFileStore(path string, keyMaterial string) *fileStore {
	sum := sha256.Sum256([]byte("vessel-token-key:" + keyMaterial))
	return &fileStore{path: path, key: sum[:]}
}

func (f *fileStore) Save(refresh string) error {
	if refresh == "" {
		return f.Delete()
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(refresh), nil)

	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, sealed, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func (f *fileStore) Load() (string, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(f.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("token file corrupt")
	}
	nonce, ct := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (f *fileStore) Delete() error {
	err := os.Remove(f.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// fallbackStore tries keyring first and falls back to the encrypted file when
// the keyring is unavailable (headless Linux, no secret service, etc.).
type fallbackStore struct {
	primary  TokenStore
	fallback TokenStore
}

// NewStore builds the production token store: OS keyring with an encrypted-file
// fallback at filePath. keyMaterial should be stable per machine/user.
func NewStore(filePath, keyMaterial string) TokenStore {
	return &fallbackStore{
		primary:  keyringStore{},
		fallback: newFileStore(filePath, keyMaterial),
	}
}

func (s *fallbackStore) Save(refresh string) error {
	if err := s.primary.Save(refresh); err != nil {
		return s.fallback.Save(refresh)
	}
	return nil
}

func (s *fallbackStore) Load() (string, error) {
	if v, err := s.primary.Load(); err == nil && v != "" {
		return v, nil
	}
	return s.fallback.Load()
}

func (s *fallbackStore) Delete() error {
	e1 := s.primary.Delete()
	e2 := s.fallback.Delete()
	if e1 != nil {
		return e1
	}
	return e2
}
