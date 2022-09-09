// Copyright (c) 2021 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0, or the Apache License, Version 2.0
// which is available at https://www.apache.org/licenses/LICENSE-2.0.
//
// SPDX-License-Identifier: EPL-2.0 OR Apache-2.0

package tpmtls

import (
	"bytes"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"github.com/pkg/errors"

	"github.com/ThreeDotsLabs/watermill"

	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

// ContextOpts defines TLS options.
type ContextOpts struct {
	// TPMConnectionRW This MUST be initialized with an io.ReadWriter returned by Connection.GetRW
	TPMConnectionRW io.ReadWriter

	PrivateKeyFile       string
	PublicKeyFile        string
	StorageRootKeyHandle uint32

	PublicCertFile string

	ExtTLSConfig *tls.Config
}

// Context represents TPM context.
type Context interface {
	crypto.Signer
	crypto.Decrypter
	// Close When you finish using this context, generally whenever you close the TLS connection
	// you need to close it and release the io.ReadWriter object it returns.
	Close() (io.ReadWriter, error)

	// TLSConfig returns an tls.Config object that can be used to establish TSL connection using
	// this TPM context.
	TLSConfig() *tls.Config
}

type tpmTLSContext struct {
	cfg              ContextOpts
	x509Certificate  x509.Certificate
	publicKey        *tpm2.Public
	cachedKeyContext tpmutil.Handle

	genLock *sync.RWMutex

	logger watermill.LoggerAdapter
}

// NewTPMContext creates a TPM context using the provided ContextOps.
func NewTPMContext(opts *ContextOpts, logger watermill.LoggerAdapter) (context Context, err error) {
	if opts.TPMConnectionRW == nil {
		return nil, errors.New("the TPM connection stream is required")
	}
	if opts.StorageRootKeyHandle == 0 {
		return nil, errors.New("the TPM Storage Root Key handle is required")
	}
	if len(opts.PublicKeyFile) == 0 {
		return nil, errors.New("the TPM Public Key File is required")
	}
	if len(opts.PrivateKeyFile) == 0 {
		return nil, errors.New("the TPM Private Key File is required")
	}
	if len(opts.PublicCertFile) == 0 {
		return nil, errors.New("the Public Certificate File is required")
	}
	if opts.ExtTLSConfig == nil {
		return nil, errors.New("the External TLS Config is required")
	}
	if len(opts.ExtTLSConfig.Certificates) > 0 {
		return nil, errors.New("Certificates value in ExtTLSConfig MUST be empty")
	}
	if len(opts.ExtTLSConfig.CipherSuites) > 0 {
		return nil, errors.New("CipherSuites value in ExtTLSConfig MUST be empty")
	}

	ctx := tpmTLSContext{
		cfg: ContextOpts{
			TPMConnectionRW:      opts.TPMConnectionRW,
			PrivateKeyFile:       opts.PrivateKeyFile,
			PublicKeyFile:        opts.PublicKeyFile,
			StorageRootKeyHandle: opts.StorageRootKeyHandle,
			PublicCertFile:       opts.PublicCertFile,
			ExtTLSConfig:         opts.ExtTLSConfig,
		},
		genLock: &sync.RWMutex{},
		logger:  logger,
	}
	err = ctx.validateAndLoadPublicCert()
	if err != nil {
		return
	}
	err = ctx.loadKeysContext()
	if err != nil {
		return
	}
	err = ctx.loadPubKey()
	if err != nil {
		return
	}

	context = ctx
	return
}

// Sign signs the defined context data.
func (t tpmTLSContext) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	t.genLock.RLock()
	defer t.genLock.RUnlock()

	if t.cfg.TPMConnectionRW == nil {
		return nil, errors.New("this TPM TLS Context is closed")
	}
	hash := opts.HashFunc()

	if len(digest) != hash.Size() {
		return nil, errors.New("sal: Sign: Digest length doesn't match passed crypto algorithm")
	}

	tpmHash := tpm2.AlgSHA256

	/*
		todo
		  At some point we might want to support more algorithms.
		  currently in the object returned by TLSConfig() SHA256 is hardcoded as the only supported
		  hash algorithm see the comments in TLSConfig for explanation why

		switch hash.HashFunc() {
		case crypto.SHA256:
			tpmHash = tpm2.AlgSHA256
		case crypto.SHA384:
			tpmHash = tpm2.AlgSHA384
		case crypto.SHA512:
			tpmHash = tpm2.AlgSHA256
		}
	*/

	if hash.HashFunc() != crypto.SHA256 {
		return nil,
			errors.Errorf("sal: Sign: Hash function '%v' is not supported, only '%v' is supported", hash, crypto.SHA256)
	}

	sig, err := tpm2.Sign(t.cfg.TPMConnectionRW, t.cachedKeyContext, "", digest, nil, &tpm2.SigScheme{
		Alg:  tpm2.AlgRSAPSS,
		Hash: tpmHash,
	})
	if err != nil {
		return nil, errors.Errorf("sign with TPM failed: %v", err)
	}
	return sig.RSA.Signature, nil
}

// Public returns the public key.
func (t tpmTLSContext) Public() (pubKey crypto.PublicKey) {
	t.genLock.RLock()
	defer t.genLock.RUnlock()

	if t.cfg.TPMConnectionRW == nil {
		return
	}
	t.logger.Debug("Get Public key", nil)
	pubKey, _ = t.publicKey.Key()
	return pubKey
}

func (t tpmTLSContext) Decrypt(_ io.Reader, msg []byte, _ crypto.DecrypterOpts) (plaintext []byte, err error) {
	t.genLock.RLock()
	defer t.genLock.RUnlock()

	t.logger.Debug("Decrypt", nil)

	if t.cfg.TPMConnectionRW == nil {
		err = errors.New("this TPM TLS Context is closed")
		return
	}

	scheme := &tpm2.AsymScheme{
		Alg:  tpm2.AlgOAEP,
		Hash: tpm2.AlgSHA256,
	}
	return tpm2.RSADecrypt(t.cfg.TPMConnectionRW, t.cachedKeyContext, "", msg, scheme, "")
}

// Close closes the TPM context.
func (t tpmTLSContext) Close() (rw io.ReadWriter, err error) {
	t.genLock.Lock()
	defer t.genLock.Unlock()

	t.logger.Debug("Closing TPM TLS context!", nil)

	rw = t.cfg.TPMConnectionRW
	if rw == nil {
		err = errors.New("this TPM TLS Context is already closed")
	} else {
		_ = tpm2.FlushContext(t.cfg.TPMConnectionRW, t.cachedKeyContext)
		t.publicKey = nil
		t.cachedKeyContext = 0
		t.cfg.TPMConnectionRW = nil
	}

	return
}

// TLSConfig returns the TLS Config.
func (t tpmTLSContext) TLSConfig() (cfg *tls.Config) {
	t.genLock.RLock()
	defer t.genLock.RUnlock()
	if t.cfg.TPMConnectionRW == nil {
		return
	}
	tlsCert := tls.Certificate{
		PrivateKey:  t,
		Leaf:        &t.x509Certificate,
		Certificate: [][]byte{t.x509Certificate.Raw},
	}
	cfg = &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		RootCAs:            t.cfg.ExtTLSConfig.RootCAs,
		InsecureSkipVerify: t.cfg.ExtTLSConfig.InsecureSkipVerify,
		/*
			TODO
			  We specify only SHA265 here because we observed undesirable behavior from the TPM
			  integrated in CtrlX if anything else is requested by the server.
			  With other hardware we might need to adjust this
		*/
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		},
		MaxVersion: tls.VersionTLS12,
	}
	return
}

func (t *tpmTLSContext) validateAndLoadPublicCert() error {
	t.genLock.Lock()
	defer t.genLock.Unlock()

	t.logger.Debug("Loading the certificate file.... ", nil)
	pubPEM, err := ioutil.ReadFile(t.cfg.PublicCertFile)
	if err != nil {
		return errors.Wrap(err, "unable to read certificate file content")
	}

	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return errors.New("unable to decode certificate file content")
	}

	pub, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "unable to parse certificate file content")
	}

	t.logger.Debug("Certificate loaded", nil)
	t.x509Certificate = *pub

	return nil
}

func (t *tpmTLSContext) loadKeysContext() error {
	t.genLock.Lock()
	defer t.genLock.Unlock()
	if t.cachedKeyContext == 0 {
		t.logger.Debug("Trying to convert the keys", nil)

		privBytes, err := ioutil.ReadFile(t.cfg.PrivateKeyFile)
		if err != nil {
			return err
		}

		pubBytes, err := ioutil.ReadFile(t.cfg.PublicKeyFile)
		if err != nil {
			return err
		}

		tpmPubBlob := tpmutil.U16Bytes(pubBytes)
		buf := bytes.NewBuffer(tpmPubBlob)
		err = tpmPubBlob.TPMUnmarshal(buf)
		if err != nil {
			return err
		}

		tpmPrivBlob := tpmutil.U16Bytes(privBytes)
		buf = bytes.NewBuffer(tpmPrivBlob)
		err = tpmPrivBlob.TPMUnmarshal(buf)
		if err != nil {
			return err
		}

		parentHandle := tpmutil.Handle(t.cfg.StorageRootKeyHandle)
		handle, _, err := tpm2.Load(t.cfg.TPMConnectionRW, parentHandle, "", tpmPubBlob, tpmPrivBlob)
		if err != nil {
			_ = tpm2.FlushContext(t.cfg.TPMConnectionRW, handle)
			return errors.Wrap(err, "tpm2.Load failed")
		}

		t.cachedKeyContext = handle
		t.logger.Debug(fmt.Sprintf("cachedKeyContext set to 0x%X", handle), nil)
	}

	return nil
}

func (t *tpmTLSContext) loadPubKey() error {
	t.genLock.Lock()
	defer t.genLock.Unlock()

	t.logger.Debug("Invoked Public()", nil)
	if t.publicKey == nil {
		t.logger.Debug(fmt.Sprintf("cachedKeyContext set to 0x%X", t.cachedKeyContext), nil)

		rw := t.cfg.TPMConnectionRW
		handle := t.cachedKeyContext
		pub, _, _, err := tpm2.ReadPublic(rw, handle)
		if err != nil {
			return err
		}
		//log.Debug("PublicArea: \n%s", log.SpewS(pub))
		pubKey, err := pub.Key()
		if err != nil {
			return err
		}
		t.publicKey = &pub
		t.logger.Debug(fmt.Sprintf("Acquired public key from tpm: %v", pubKey), nil)
	}

	return nil
}
