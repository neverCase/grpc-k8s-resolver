package env

import (
	"errors"
	"os"
)

const (
	StaticCaFile     = "TLS_OPTION_CA_FILE"
	StaticServerName = "TLS_OPTION_SERVER_NAME"
)

var ErrGrpcTLSNotInCluster = errors.New("unable to load tls configuration, TLS_OPTION_CA_FILE or TLS_OPTION_SERVER_NAME must be defined")

func StaticClientCerts() (string, string, error) {
	caFile, serverName := os.Getenv(StaticCaFile), os.Getenv(StaticServerName)
	if len(caFile) == 0 || len(serverName) == 0 {
		return "", "", ErrGrpcTLSNotInCluster
	}
	return caFile, serverName, nil
}
