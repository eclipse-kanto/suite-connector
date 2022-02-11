// Copyright (c) 2021 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// http://www.eclipse.org/legal/epl-2.0
//
// SPDX-License-Identifier: EPL-2.0

package tpmtls

import (
	"io"
	"net"
	"os"
	"sync"

	"github.com/pkg/errors"

	"github.com/google/go-tpm/tpm2"
)

// Connection This interface represents an OPEN connection to the TPM module see NewTpmConnection
// for information how to obtain an instance.
//
// Due to inconsistencies between how device files and unix sockets are handled by tpm2.OpenTPM
// and the tpm2 library as a whole we need to handle this our self.
//
// For all cases to work as expected the unix socket connection must be kept open until
// we no longer need it. If we close the connection and reopen it we nee to do the whole
// initialization procedure again. In order to avoid that we keep the connection open
// until explicit close request.
// Also GetRW and ReleaseRW methods provide some rudimentary ownership management
type Connection interface {
	// Close Closes the connection.
	//       Note if the io.ReadWriter returned by GetRW is not released
	//       it's not possible to close the connection.
	Close() error
	// GetRW Returns io.ReadWriter object that can be used to communicate with the TPM
	//       For example you can pass it to all tpm2 functions
	//       Note if the io.ReadWriter must be released using the ReleaseRW method
	//       before Close method is invoked.
	GetRW() (io.ReadWriter, error)
	// ReleaseRW Releases the io.ReadWriter received by the GetRW method
	ReleaseRW(writer io.ReadWriter) error
}

type tpmConnection struct {
	deviceFile string

	rwc     io.ReadWriteCloser
	rw      io.ReadWriter
	rwMutex *sync.Mutex
}

type rwProxy struct {
	rwc io.ReadWriteCloser
}

// NewTpmConnection creates connection for a TPM device file name.
func NewTpmConnection(fileName string) (conn Connection, err error) {
	connection := tpmConnection{
		deviceFile: fileName,
		rwMutex:    &sync.Mutex{},
	}

	var rwc io.ReadWriteCloser
	var fi os.FileInfo
	fi, err = os.Stat(fileName)
	if err != nil {
		return
	}

	if fi.Mode()&os.ModeDevice != 0 {
		var f *os.File
		f, err = os.OpenFile(fileName, os.O_RDWR, 0600)
		if err != nil {
			return
		}
		rwc = io.ReadWriteCloser(f)
	} else if fi.Mode()&os.ModeSocket != 0 {
		rwc, err = net.Dial("unix", fileName)
		if err != nil {
			return
		}
	} else {
		return nil, errors.Errorf("unsupported TPM file mode %s", fi.Mode().String())
	}
	// Make sure this is a TPM 2.0
	_, err = tpm2.GetManufacturer(rwc)
	if err != nil {
		rwc.Close()
		return nil, errors.Errorf("open %s: device is not a TPM 2.0", fileName)
	}
	connection.rwc = rwc
	conn = connection
	return
}

func (r rwProxy) Read(p []byte) (int, error) {
	return r.rwc.Read(p)
}

func (r rwProxy) Write(p []byte) (int, error) {
	return r.rwc.Write(p)
}

func (t tpmConnection) Close() error {
	t.rwMutex.Lock()
	defer t.rwMutex.Unlock()
	if t.rw != nil {
		return errors.New("can't close the TPM connection it's currently in use")
	}
	t.rwc.Close()
	t.rwc = nil
	return nil
}

func (t tpmConnection) ReleaseRW(writer io.ReadWriter) (err error) {
	t.rwMutex.Lock()
	defer t.rwMutex.Unlock()
	if writer == t.rw {
		t.rw = nil
		return
	}
	err = errors.New("this is RW is not the expected one")
	return
}

func (t tpmConnection) GetRW() (io.ReadWriter, error) {
	t.rwMutex.Lock()
	defer t.rwMutex.Unlock()
	if t.rw == nil {
		t.rw = rwProxy{
			rwc: t.rwc,
		}
		return t.rw, nil
	}
	return nil, errors.New("tpm RW is currently unavailable")
}
