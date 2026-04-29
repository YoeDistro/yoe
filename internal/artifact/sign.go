package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Signer carries the loaded RSA private key and the metadata apk-tools
// needs to identify the matching public key on-target.
//
// KeyName is the file name as it lives in /etc/apk/keys/ on the booted
// system — e.g., "myproj.rsa.pub". The signature tar entry is named
// `.SIGN.RSA.<KeyName>`, matching apk-tools 2.x's verification path: when
// apk reads `.SIGN.RSA.foo.rsa.pub`, it loads `/etc/apk/keys/foo.rsa.pub`
// and verifies the signature against that key.
//
// PubPEM holds the PEM-encoded SubjectPublicKeyInfo (the "PUBLIC KEY"
// PEM block) — the same format Alpine ships in /etc/apk/keys/. Callers
// publish it next to the repo and into the booted rootfs so apk verifies
// signatures without --allow-untrusted.
type Signer struct {
	Key     *rsa.PrivateKey
	KeyName string
	PubPEM  []byte
}

// LoadOrGenerateSigner returns a Signer for the given project. If
// configuredPath is set (signing_key on project()), the key is loaded
// from there; otherwise yoe defaults to ~/.config/yoe/keys/<project>.rsa
// and generates a fresh 2048-bit RSA keypair if none exists.
//
// The matching public key is always written to <privatePath>.pub. This
// is the canonical source of truth — image-time apk add reads it via
// --keys-dir, and the base-files unit ships a copy into the rootfs at
// /etc/apk/keys/<keyname>.rsa.pub.
func LoadOrGenerateSigner(projectName, configuredPath string) (*Signer, error) {
	privPath, err := resolveKeyPath(projectName, configuredPath)
	if err != nil {
		return nil, err
	}
	pubPath := privPath + ".pub"

	priv, err := loadOrCreatePrivateKey(privPath)
	if err != nil {
		return nil, err
	}

	pubPEM, err := loadOrWritePublicKey(pubPath, &priv.PublicKey)
	if err != nil {
		return nil, err
	}

	return &Signer{
		Key:     priv,
		KeyName: filepath.Base(pubPath),
		PubPEM:  pubPEM,
	}, nil
}

func resolveKeyPath(projectName, configuredPath string) (string, error) {
	if configuredPath != "" {
		return configuredPath, nil
	}
	if projectName == "" {
		return "", fmt.Errorf("cannot derive default signing key path: project name is empty")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving signing key path: %w (set signing_key on project() to override)", err)
	}
	return filepath.Join(home, ".config", "yoe", "keys", projectName+".rsa"), nil
}

func loadOrCreatePrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("parsing %s: not a PEM block", path)
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(block.Bytes)
		case "PRIVATE KEY":
			k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("parsing %s: %w", path, err)
			}
			rk, ok := k.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("parsing %s: not an RSA key", path)
			}
			return rk, nil
		default:
			return nil, fmt.Errorf("parsing %s: unexpected PEM type %q", path, block.Type)
		}
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	// Generate fresh.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("creating key dir: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return priv, nil
}

func loadOrWritePublicKey(path string, pub *rsa.PublicKey) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("encoding public key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})
	if err := os.WriteFile(path, pemBytes, 0644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}
	return pemBytes, nil
}

// SignStream returns the gzipped signature stream for `data` — the bytes
// to prepend in front of the apk's control stream (or APKINDEX) to make a
// signed concatenated archive. The signature is RSA-PKCS#1 v1.5 over the
// SHA-1 of `data`, matching apk-tools 2.x's RSA verification.
func (s *Signer) SignStream(data []byte) ([]byte, error) {
	digest := sha1.Sum(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.Key, crypto.SHA1, digest[:])
	if err != nil {
		return nil, fmt.Errorf("signing: %w", err)
	}
	return s.signatureGzipStream(sig)
}

// signatureGzipStream wraps the signature bytes in a single-entry tar
// (entry name = `.SIGN.RSA.<keyname>`) inside a gzip stream. The tar is
// flushed without a trailer — apk reads exactly one gzip stream at a time,
// so the standard 2-block tar EOF marker would just be wasted bytes. The
// header shape mirrors what writeGzipTar in apk.go uses for the control
// stream: bare Name/Size/Mode/ModTime, no PaX records, no Typeflag set.
// apk's signature parser is order-tolerant on tar fields but rejects
// unexpected extended headers in some configurations.
func (s *Signer) signatureGzipStream(signature []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name:    ".SIGN.RSA." + s.KeyName,
		Mode:    0644,
		Size:    int64(len(signature)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(signature); err != nil {
		return nil, err
	}
	if err := tw.Flush(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
