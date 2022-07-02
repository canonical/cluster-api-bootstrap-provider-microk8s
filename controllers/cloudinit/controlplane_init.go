/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudinit

import (
	"net"

	"sigs.k8s.io/cluster-api/util/secret"
)

const (
	controlPlaneCloudInit = `{{.Header}}
write_files:
- content: |
    -----BEGIN RSA PRIVATE KEY-----
    MIIEowIBAAKCAQEAqcYDwsBmJLJmkex9m44x+I2T7CyoJQoCZxQAHkOD3b68s0OH
    UkxumXZe0XvF9TPW/rGKeoVAEJJ0nwxzDcs8fVzMp4RfuKPh/cfTNeRMkbwXX1aX
    1MpkO51G7DZm9lotmZ8CUzNvCi9pS0aO35YODphxPkqOkXjdnBnAXHXWVPRjw6zP
    hXcwWAZU05el8iHU8ZMo2Y9yXZ6xiqi6gMesztJX1C1URYzqtoQOySk8slmEq1sQ
    QD161SLVlguD7/VHnKHfylBrfQbiG/pi1fnAvxWbmY8moshiZn1sCk33o764OwAy
    8PSCTIr5rg9byMIvsHw6DUgOjzIcJU6kuugLQwIDAQABAoIBADJ7JZ0gFn8s6ODV
    ABhj9KnidyyPrsOssUAzK0HUc35Y+8UV/EPVZEGPd+w0MI2Th4ceBX4e5wjGc5Tj
    X8anOupP0K6y5r+BQ25xn3Tz2GyxEAYSOn1UXO94+aC9IGp6L/rw1AEnVwohRN7U
    MSF8fduLKokKJFBPLx3+bjtP8pY5xPix0wCJ/iC/CNNdYzvxpSkg2xnzIGN6/IcP
    Lb38DpSYOI35qe68jue/82DO+tBIIvy9rDluxULZSn4IX/qapmB8noldRVSYkyaa
    tcs95SGubC5mYblLDlQZLrurNDqo8lp8ktugiRhBFoQgabverNPBI/bmvfndpDXw
    JaM5DTkCgYEA1XwznKMHTIa4kZviqFwm0ug68Bn/AlNzuf7Yg5oiwoSyS3dkJ16v
    FSDe7xKh93N/Qws1Wq0YtEM1om1NVR1VW2bPg/wT9Y9Lg9VlueHCzjytPqz+j88i
    E6nIy+FVD+vG9pyPqOHl8j03xQqDNyDzynn0/fKNCykIoeGOG0EyPt0CgYEAy5VT
    ZQKpf/eLF271UH0nOcDayXRaI11LKmK2LfqTW1ZzRNyofemG4q2VG7WapwicVpte
    sqgwa0XsBrMM87Tp7GfO0xAsUcBON3DD6+nn/T90VSLdmK7vSdJmr60/yyXDpE39
    Y6gmXt1c3UMYxrWYPFM7op/uF2YP3AkT5YjPAJ8CgYB/2/JBZvbRI4LZWoamlQJ/
    oKzj7n3nk7mk9PgR4bfdzoHGZwwp9DBiNByxDPTKcncO3WCoHTHFjNdLn7EIQBhG
    NM4mW0xM7vSoUZ+qc4cr4/VSq2OPF9xt8GsdiKhcb7brLptv51PEAFwte/1YgDji
    1KYhjiphO8M6yQ9GTYbdVQKBgQCc+Abz9CiC3XfmWoxVQhpTgmpvOAIkEFPbW38C
    Vpj1rON1rflQFBYHgzVbxxt2PMJmWKeccufaXnBM/hM3eT+AIs4qmObDJdZpEs5N
    gO15q0pkNlzL0932en7oZ1mvpe+CKQv9ofHr5RwsEgbxd6TopnhtvIhUjEIgMvOf
    YGvTGwKBgDdd38lkcv0ItR9DblqYghukgcLkKa7IU61hedSOWvDD3lXLq9fedpv5
    vQDaqoWDrhUbrlndC1dtdFjclHmLNwbUxlIRKTZF1nqZWBh/Wg/TUvuhpANtK1TM
    7juq88gjsqFcbFg3c87Tq8yVDBopuXK93sOecUqkQaLsZiaCZYSQ
    -----END RSA PRIVATE KEY-----
  path: /var/tmp/ca.key
  permissions: '0600'
- content: | 
    -----BEGIN CERTIFICATE-----
    MIIDDzCCAfegAwIBAgIUBspw4RmsMMIEDpISPGr9LHu2fX0wDQYJKoZIhvcNAQEL
    BQAwFzEVMBMGA1UEAwwMMTAuMTUyLjE4My4xMB4XDTIyMDYxNzEyMjUxOVoXDTMy
    MDYxNDEyMjUxOVowFzEVMBMGA1UEAwwMMTAuMTUyLjE4My4xMIIBIjANBgkqhkiG
    9w0BAQEFAAOCAQ8AMIIBCgKCAQEAqcYDwsBmJLJmkex9m44x+I2T7CyoJQoCZxQA
    HkOD3b68s0OHUkxumXZe0XvF9TPW/rGKeoVAEJJ0nwxzDcs8fVzMp4RfuKPh/cfT
    NeRMkbwXX1aX1MpkO51G7DZm9lotmZ8CUzNvCi9pS0aO35YODphxPkqOkXjdnBnA
    XHXWVPRjw6zPhXcwWAZU05el8iHU8ZMo2Y9yXZ6xiqi6gMesztJX1C1URYzqtoQO
    ySk8slmEq1sQQD161SLVlguD7/VHnKHfylBrfQbiG/pi1fnAvxWbmY8moshiZn1s
    Ck33o764OwAy8PSCTIr5rg9byMIvsHw6DUgOjzIcJU6kuugLQwIDAQABo1MwUTAd
    BgNVHQ4EFgQUPDAHKb9FT7M8BBDI7YHmT3FvgOYwHwYDVR0jBBgwFoAUPDAHKb9F
    T7M8BBDI7YHmT3FvgOYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOC
    AQEAHveyd0stFhqcTDOgOG0rZXW0R3KDlpW766FADdp96yPdDxoqggTBKkh0cgvz
    anoj1GBf+FEYQnF9IoosmqYXyEU3eSNLSc56ei51WyeCkPQ9OQfvOz9QSkCOiUrH
    ExsW9PE6W90cIRz3HzExGyKE+RCeCjfZCVcmMAW0EkuUrxt/crIzhqJQa5BNWXFs
    6pESYAIlkqcDNgyIn8aUc1+hJuWQjlWxIQsqgOOF4nzf9KLcBzMlCD4pW8zFVEx4
    XUXB56XrXgJ32In0q+N9tyWVCwI6KuF1jO5nXjSLCA4PvJArh7b1l7iI8fqGOrit
    taJNTy7Zp91BrPV7sakP5xT+mw==
    -----END CERTIFICATE-----
  path: /var/tmp/ca.crt
  permissions: '0600'
runcmd:
- sudo iptables -t nat -A OUTPUT -o lo -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo iptables -A PREROUTING -t nat  -p tcp --dport 6443 -j REDIRECT --to-port 16443
- sudo apt-get update
- sudo apt-get install iptables-persistent
- sudo snap install microk8s --classic
- sudo microk8s refresh-certs /var/tmp
- sudo sleep 30
- sudo sed -i '/^DNS.1 = kubernetes/a {{.ControlPlaneEndpointType}}.100 = {{.ControlPlaneEndpoint}}' /var/snap/microk8s/current/certs/csr.conf.template
- sudo microk8s status --wait-ready
- sudo microk8s add-node --token-ttl 86400 --token {{.JoinToken}}
- sudo microk8s enable dns
- sudo sleep 15
`
)

// ControlPlaneInput defines the context to generate a controlplane instance user data.
type ControlPlaneInput struct {
	BaseUserData
	secret.Certificates
	ControlPlaneEndpoint     string
	ControlPlaneEndpointType string
	JoinToken                string
	ClusterConfiguration     string
	InitConfiguration        string
}

// NewInitControlPlane returns the user data string to be used on a controlplane instance.
func NewInitControlPlane(input *ControlPlaneInput) ([]byte, error) {
	input.Header = cloudConfigHeader
	input.WriteFiles = input.Certificates.AsFiles()
	input.WriteFiles = append(input.WriteFiles, input.AdditionalFiles...)
	input.SentinelFileCommand = sentinelFileCommand
	input.ControlPlaneEndpointType = "DNS"
	addr := net.ParseIP(input.ControlPlaneEndpoint)
	if addr != nil {
		input.ControlPlaneEndpointType = "IP"
	}

	userData, err := generate("InitControlplane", controlPlaneCloudInit, input)
	if err != nil {
		return nil, err
	}

	return userData, nil
}
