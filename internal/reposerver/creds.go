package reposerver

import (
	"encoding/json"

	"github.com/orin/orin/internal/domain"
)

// EncodeCreds serialises decrypted credentials for storage.
func EncodeCreds(c *domain.RepoCreds) ([]byte, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// decodeCreds is the inverse.
func decodeCreds(b []byte, c *domain.RepoCreds) error {
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, c)
}
