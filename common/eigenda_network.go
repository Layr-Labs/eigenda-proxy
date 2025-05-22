package common

import "fmt"

type EigenDANetwork string

const (
	HoleskyTestnetEigenDANetwork EigenDANetwork = "holesky_testnet"
	HoleskyPreprodEigenDANetwork EigenDANetwork = "holesky_preprod"
)

// GetLegacyCertVerifierAddress returns, as a string, the address of the LegacyEigenDACertVerifier contract for the network.
func (n EigenDANetwork) GetLegacyCertVerifierAddress() (string, error) {
	switch n {
	case HoleskyTestnetEigenDANetwork:
		return "0xFe52fE1940858DCb6e12153E2104aD0fDFbE1162", nil
	case HoleskyPreprodEigenDANetwork:
		return "0xd973fA62E22BC2779F8489258F040C0344B03C21", nil
	default:
		return "", fmt.Errorf("unknown network type: %s", n)
	}
}

// GetServiceManagerAddress returns, as a string, the address of the EigenDAServiceManager contract for the network.
func (n EigenDANetwork) GetServiceManagerAddress() (string, error) {
	switch n {
	case HoleskyTestnetEigenDANetwork:
		return "0xD4A7E1Bd8015057293f0D0A557088c286942e84b", nil
	case HoleskyPreprodEigenDANetwork:
		return "0x54A03db2784E3D0aCC08344D05385d0b62d4F432", nil
	default:
		return "", fmt.Errorf("unknown network type: %s", n)
	}
}

// GetDisperserAddress gets a string representing the address of the disperser for the network.
// The format of the returned address is "<hostname>:<port>"
func (n EigenDANetwork) GetDisperserAddress() (string, error) {
	switch n {
	case HoleskyTestnetEigenDANetwork:
		return "disperser-testnet-holesky.eigenda.xyz:443", nil
	case HoleskyPreprodEigenDANetwork:
		return "disperser-preprod-holesky.eigenda.xyz:443", nil
	default:
		return "", fmt.Errorf("unknown network type: %s", n)
	}
}

// GetBLSOperatorStateRetrieverAddress returns, as a string, the address of the OperatorStateRetriever contract for the
// network
func (n EigenDANetwork) GetBLSOperatorStateRetrieverAddress() (string, error) {
	switch n {
	case HoleskyTestnetEigenDANetwork, HoleskyPreprodEigenDANetwork:
		return "0x003497Dd77E5B73C40e8aCbB562C8bb0410320E7", nil
	default:
		return "", fmt.Errorf("unknown network: %s", n)
	}
}

// GetRegistryCoordinatorAddress returns, as a string, the address of the RegistryCoordinator contract for the
// network
func (n EigenDANetwork) GetRegistryCoordinatorAddress() (string, error) {
	switch n {
	case HoleskyTestnetEigenDANetwork:
		return "0x53012C69A189cfA2D9d29eb6F19B32e0A2EA3490", nil
	case HoleskyPreprodEigenDANetwork:
		return "0x2c61EA360D6500b58E7f481541A36B443Bc858c6", nil
	default:
		return "", fmt.Errorf("unknown network: %s", n)
	}
}

func (n EigenDANetwork) String() string {
	return string(n)
}

func EigenDANetworkFromString(inputString string) (EigenDANetwork, error) {
	network := EigenDANetwork(inputString)

	switch network {
	case HoleskyTestnetEigenDANetwork, HoleskyPreprodEigenDANetwork:
		return network, nil
	default:
		return "", fmt.Errorf("unknown network type: %s", inputString)
	}
}
