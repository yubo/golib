// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package configtls

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptionsToConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     TLSSetting
		expectError string
	}{
		{
			name:    "should load system CA",
			options: TLSSetting{CAFile: ""},
		},
		{
			name:    "should load custom CA",
			options: TLSSetting{CAFile: filepath.Join("testdata", "ca-1.crt")},
		},
		{
			name:        "should fail with invalid CA file path",
			options:     TLSSetting{CAFile: filepath.Join("testdata", "not/valid")},
			expectError: "failed to load CA",
		},
		{
			name:        "should fail with invalid CA file content",
			options:     TLSSetting{CAFile: filepath.Join("testdata", "testCA-bad.txt")},
			expectError: "failed to parse CA",
		},
		{
			name: "should load valid TLS  settings",
			options: TLSSetting{
				CAFile:   filepath.Join("testdata", "ca-1.crt"),
				CertFile: filepath.Join("testdata", "server-1.crt"),
				KeyFile:  filepath.Join("testdata", "server-1.key"),
			},
		},
		{
			name: "should fail with missing TLS KeyFile",
			options: TLSSetting{
				CAFile:   filepath.Join("testdata", "ca-1.crt"),
				CertFile: filepath.Join("testdata", "server-1.crt"),
			},
			expectError: "both certificate and key must be supplied",
		},
		{
			name: "should fail with invalid TLS KeyFile",
			options: TLSSetting{
				CAFile:   filepath.Join("testdata", "ca-1.crt"),
				CertFile: filepath.Join("testdata", "server-1.crt"),
				KeyFile:  filepath.Join("testdata", "not/valid"),
			},
			expectError: "failed to load TLS cert and key",
		},
		{
			name: "should fail with missing TLS Cert",
			options: TLSSetting{
				CAFile:  filepath.Join("testdata", "ca-1.crt"),
				KeyFile: filepath.Join("testdata", "server-1.key"),
			},
			expectError: "both certificate and key must be supplied",
		},
		{
			name: "should fail with invalid TLS Cert",
			options: TLSSetting{
				CAFile:   filepath.Join("testdata", "ca-1.crt"),
				CertFile: filepath.Join("testdata", "not/valid"),
				KeyFile:  filepath.Join("testdata", "server-1.key"),
			},
			expectError: "failed to load TLS cert and key",
		},
		{
			name: "should fail with invalid TLS CA",
			options: TLSSetting{
				CAFile: filepath.Join("testdata", "not/valid"),
			},
			expectError: "failed to load CA",
		},
		{
			name: "should fail with invalid CA pool",
			options: TLSSetting{
				CAFile: filepath.Join("testdata", "testCA-bad.txt"),
			},
			expectError: "failed to parse CA",
		},
		{
			name: "should pass with valid CA pool",
			options: TLSSetting{
				CAFile: filepath.Join("testdata", "ca-1.crt"),
			},
		},
		{
			name: "should pass with valid min and max version",
			options: TLSSetting{
				MinVersion: "1.1",
				MaxVersion: "1.2",
			},
		},
		{
			name: "should pass with invalid min",
			options: TLSSetting{
				MinVersion: "1.7",
			},
			expectError: "invalid TLS min_",
		},
		{
			name: "should pass with invalid max",
			options: TLSSetting{
				MaxVersion: "1.7",
			},
			expectError: "invalid TLS max_",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := test.options.loadTLSConfig()
			if test.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectError)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, cfg)
			}
		})
	}
}

func TestLoadTLSClientConfigError(t *testing.T) {
	tlsSetting := TLSClientSetting{
		TLSSetting: TLSSetting{
			CertFile: "doesnt/exist",
			KeyFile:  "doesnt/exist",
		},
	}
	_, err := tlsSetting.LoadTLSConfig()
	assert.Error(t, err)
}

func TestLoadTLSClientConfig(t *testing.T) {
	tlsSetting := TLSClientSetting{
		Insecure: true,
	}
	tlsCfg, err := tlsSetting.LoadTLSConfig()
	assert.NoError(t, err)
	assert.Nil(t, tlsCfg)

	tlsSetting = TLSClientSetting{}
	tlsCfg, err = tlsSetting.LoadTLSConfig()
	assert.NoError(t, err)
	assert.NotNil(t, tlsCfg)

	tlsSetting = TLSClientSetting{
		InsecureSkipVerify: true,
	}
	tlsCfg, err = tlsSetting.LoadTLSConfig()
	assert.NoError(t, err)
	assert.NotNil(t, tlsCfg)
	assert.True(t, tlsCfg.InsecureSkipVerify)
}

func TestLoadTLSServerConfigError(t *testing.T) {
	tlsSetting := TLSServerSetting{
		TLSSetting: TLSSetting{
			CertFile: "doesnt/exist",
			KeyFile:  "doesnt/exist",
		},
	}
	_, err := tlsSetting.LoadTLSConfig()
	assert.Error(t, err)

	tlsSetting = TLSServerSetting{
		ClientCAFile: "doesnt/exist",
	}
	_, err = tlsSetting.LoadTLSConfig()
	assert.Error(t, err)
}

func TestLoadTLSServerConfig(t *testing.T) {
	tlsSetting := TLSServerSetting{}
	tlsCfg, err := tlsSetting.LoadTLSConfig()
	assert.NoError(t, err)
	assert.NotNil(t, tlsCfg)
}

func TestEagerlyLoadCertificate(t *testing.T) {
	options := TLSSetting{
		CertFile: filepath.Join("testdata", "client-1.crt"),
		KeyFile:  filepath.Join("testdata", "client-1.key"),
	}
	cfg, err := options.loadTLSConfig()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{})
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	pCert, err := x509.ParseCertificate(cert.Certificate[0])
	assert.NoError(t, err)
	assert.NotNil(t, pCert)
	assert.ElementsMatch(t, []string{"example1"}, pCert.DNSNames)
}

func TestCertificateReload(t *testing.T) {
	tests := []struct {
		name           string
		reloadInterval time.Duration
		wait           time.Duration
		cert2          string
		key2           string
		dns1           string
		dns2           string
		errText        string
	}{
		{
			name:           "Should reload the certificate after reload-interval",
			reloadInterval: 100 * time.Microsecond,
			wait:           100 * time.Microsecond,
			cert2:          "client-2.crt",
			key2:           "client-2.key",
			dns1:           "example1",
			dns2:           "example2",
		},
		{
			name:           "Should return same cert if called before reload-interval",
			reloadInterval: 100 * time.Millisecond,
			wait:           100 * time.Microsecond,
			cert2:          "client-2.crt",
			key2:           "client-2.key",
			dns1:           "example1",
			dns2:           "example1",
		},
		{
			name:           "Should always return same cert if reload-interval is 0",
			reloadInterval: 0,
			wait:           100 * time.Microsecond,
			cert2:          "client-2.crt",
			key2:           "client-2.key",
			dns1:           "example1",
			dns2:           "example1",
		},
		{
			name:           "Should return an error if reloading fails",
			reloadInterval: 100 * time.Microsecond,
			wait:           100 * time.Microsecond,
			cert2:          "testCA-bad.txt",
			key2:           "client-2.key",
			dns1:           "example1",
			errText:        "failed to load TLS cert and key: tls: failed to find any PEM data in certificate input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Copy certs into a temp dir so we can safely modify them
			certFile, err := os.CreateTemp("", "cert")
			assert.NoError(t, err)
			defer os.Remove(certFile.Name())

			keyFile, err := os.CreateTemp("", "key")
			assert.NoError(t, err)
			defer os.Remove(keyFile.Name())

			fdc, err := os.Open(filepath.Join("testdata", "client-1.crt"))
			assert.NoError(t, err)
			_, err = io.Copy(certFile, fdc)
			assert.NoError(t, err)

			fdk, err := os.Open(filepath.Join("testdata", "client-1.key"))
			assert.NoError(t, err)
			_, err = io.Copy(keyFile, fdk)
			assert.NoError(t, err)
			assert.NoError(t, fdk.Close())

			options := TLSSetting{
				CertFile:       certFile.Name(),
				KeyFile:        keyFile.Name(),
				ReloadInterval: test.reloadInterval,
			}
			cfg, err := options.loadTLSConfig()
			assert.NoError(t, err)
			assert.NotNil(t, cfg)

			// Asssert that we loaded the original certificate
			cert, err := cfg.GetCertificate(&tls.ClientHelloInfo{})
			assert.NoError(t, err)
			assert.NotNil(t, cert)
			pCert, err := x509.ParseCertificate(cert.Certificate[0])
			assert.NoError(t, err)
			assert.NotNil(t, pCert)
			assert.Equal(t, test.dns1, pCert.DNSNames[0])

			// Change the certificate
			assert.NoError(t, certFile.Truncate(0))
			assert.NoError(t, keyFile.Truncate(0))
			_, err = certFile.Seek(0, 0)
			assert.NoError(t, err)
			_, err = keyFile.Seek(0, 0)
			assert.NoError(t, err)

			fdc2, err := os.Open(filepath.Join("testdata", test.cert2))
			assert.NoError(t, err)
			_, err = io.Copy(certFile, fdc2)
			assert.NoError(t, err)
			assert.NoError(t, fdc2.Close())

			fdk2, err := os.Open(filepath.Join("testdata", test.key2))
			assert.NoError(t, err)
			_, err = io.Copy(keyFile, fdk2)
			assert.NoError(t, err)
			assert.NoError(t, fdk2.Close())

			// Wait ReloadInterval to ensure a reload will happen
			time.Sleep(test.wait)

			// Assert that we loaded the new certificate
			cert, err = cfg.GetCertificate(&tls.ClientHelloInfo{})
			if test.errText == "" {
				assert.NoError(t, err)
				assert.NotNil(t, cert)
				pCert, err = x509.ParseCertificate(cert.Certificate[0])
				assert.NoError(t, err)
				assert.NotNil(t, pCert)
				assert.Equal(t, test.dns2, pCert.DNSNames[0])
			} else {
				assert.EqualError(t, err, test.errText)
			}
		})
	}
}
