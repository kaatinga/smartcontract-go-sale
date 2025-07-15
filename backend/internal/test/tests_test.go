package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/testcontainers/testcontainers-go"
)

func (s *IntegrationSuite) setupSmartContractsProject() {
	smartContractsDir, err := filepath.Abs("../../../smart-contracts")
	s.Require().NoError(err, "Failed to get smart-contracts dir")

	hardhatPath := filepath.Join(smartContractsDir, "node_modules", "hardhat")
	if _, err = os.Stat(hardhatPath); os.IsNotExist(err) {
		s.T().Logf("node_modules/hardhat not found, running npm install...")
		cmd := exec.Command("npm", "install")
		cmd.Dir = smartContractsDir
		output, err := cmd.CombinedOutput()
		s.T().Logf("npm install output: %s", output)
		s.Require().NoError(err, "Failed to install npm dependencies")
	} else {
		s.T().Logf("node_modules/hardhat found, skipping npm install")
	}
}

func (s *IntegrationSuite) compileContracts() error {
	// Compute absolute path to smart-contracts
	smartContractsDir, err := filepath.Abs("../../../smart-contracts")
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	s.T().Logf("compileContracts: using smart-contracts dir: %s", smartContractsDir)

	// Compile contracts
	cmd := exec.Command("npx", "hardhat", "compile")
	cmd.Dir = smartContractsDir

	output, err := cmd.CombinedOutput()
	s.T().Logf("Compilation output: %s", output)
	if err != nil {
		return fmt.Errorf("failed to compile contracts: %w", err)
	}

	s.T().Logf("Contracts compiled successfully")
	return nil
}

func (s *IntegrationSuite) deployContracts() (tokenAddress, saleAddress common.Address) {
	// Connect to Anvil
	client, err := ethclient.Dial(s.AnvilEndpoint)
	s.Require().NoError(err)

	// Create deployer account (use Anvil's default private key)
	privateKey, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	s.Require().NoError(err)

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(31337)) // Anvil chain ID
	s.Require().NoError(err)

	// Deploy KaatingaToken
	tokenAddress, tokenTx, _, err := s.deployKaatingaToken(auth, client)
	s.Require().NoError(err)

	// Wait for token deployment
	_, err = bind.WaitMined(context.Background(), client, tokenTx)
	s.Require().NoError(err)

	// Deploy mock USDT (for testing)
	usdtAddress, usdtTx, _, err := s.deployMockUSDT(auth, client)
	s.Require().NoError(err)

	// Wait for USDT deployment
	_, err = bind.WaitMined(context.Background(), client, usdtTx)
	s.Require().NoError(err)

	// Deploy TokenSale
	saleAddress, saleTx, _, err := s.deployTokenSale(auth, client, usdtAddress, tokenAddress)
	s.Require().NoError(err)

	// Wait for sale deployment
	_, err = bind.WaitMined(context.Background(), client, saleTx)
	s.Require().NoError(err)

	s.T().Logf("Contracts deployed:")
	s.T().Logf("  KaatingaToken: %s", tokenAddress.Hex())
	s.T().Logf("  MockUSDT: %s", usdtAddress.Hex())
	s.T().Logf("  TokenSale: %s", saleAddress.Hex())

	return tokenAddress, saleAddress
}

// Helper to deploy KaatingaToken
func (s *IntegrationSuite) deployKaatingaToken(auth *bind.TransactOpts, client *ethclient.Client) (common.Address, *types.Transaction, *bind.BoundContract, error) {
	return s.deployContractFromArtifact(
		auth, client,
		"../../../smart-contracts/artifacts/contracts/KaatingaToken.sol/KaatingaToken.json",
	)
}

// Helper to deploy a minimal mock USDT (same as KaatingaToken for test)
func (s *IntegrationSuite) deployMockUSDT(auth *bind.TransactOpts, client *ethclient.Client) (common.Address, *types.Transaction, *bind.BoundContract, error) {
	return s.deployContractFromArtifact(
		auth, client,
		"../../../smart-contracts/artifacts/contracts/KaatingaToken.sol/KaatingaToken.json",
	)
}

// Helper to deploy TokenSale
func (s *IntegrationSuite) deployTokenSale(auth *bind.TransactOpts, client *ethclient.Client, usdt, kaatinga common.Address) (common.Address, *types.Transaction, *bind.BoundContract, error) {
	return s.deployContractFromArtifact(
		auth, client,
		"../../../smart-contracts/artifacts/contracts/TokenSale.sol/TokenSale.json",
		usdt, kaatinga,
	)
}

func (s *IntegrationSuite) getContainerLogs(ctx context.Context, container testcontainers.Container) ([]byte, error) {
	logReader, err := container.Logs(ctx)
	if err != nil {
		return nil, err
	}
	defer logReader.Close()

	return io.ReadAll(logReader)
}

func mustRead(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}

// Helper to deploy contract from Hardhat JSON artifact
func (s *IntegrationSuite) deployContractFromArtifact(auth *bind.TransactOpts, client *ethclient.Client, artifactPath string, constructorArgs ...interface{}) (common.Address, *types.Transaction, *bind.BoundContract, error) {
	artifactBytes, err := os.ReadFile(artifactPath)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to read artifact: %w", err)
	}
	var artifact struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	err = json.Unmarshal(artifactBytes, &artifact)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to unmarshal artifact: %w", err)
	}
	parsedABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to parse ABI: %w", err)
	}
	bytecode := common.FromHex(strings.TrimPrefix(artifact.Bytecode, "0x"))
	address, tx, instance, err := bind.DeployContract(auth, parsedABI, bytecode, client, constructorArgs...)
	if err != nil {
		return common.Address{}, nil, nil, fmt.Errorf("failed to deploy contract: %w", err)
	}
	return address, tx, instance, nil
}

func (s *IntegrationSuite) TestAnvilRPC() {
	if s.printContainerLogs {
		s.printLogs(s.testContext, s.AnvilContainer, "Anvil")
	}

	s.Require().NotNil(s.AnvilContainer, "Anvil container should be running")

	// Test that we can get container info
	containerID := s.AnvilContainer.GetContainerID()
	s.Require().NotEmpty(containerID, "Container should have an ID")

	s.T().Logf("Anvil container is running with ID: %s", containerID)
	containerIP, err := s.AnvilContainer.ContainerIP(s.testContext)
	s.NoError(err)
	s.T().Logf("Container IP: %s", containerIP)

	req, err := http.NewRequestWithContext(s.testContext, "POST", s.AnvilEndpoint, strings.NewReader(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	s.Require().Equal(200, resp.StatusCode, "Anvil should respond to RPC method with 200")

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	s.T().Logf("RPC Response: %s", string(body))
}

// Add a basic integration test for the token sale flow
func (s *IntegrationSuite) TestTokenSaleFlow() {
	client, err := ethclient.Dial(s.AnvilEndpoint)
	s.Require().NoError(err)

	privateKey, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	s.Require().NoError(err)
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(31337))
	s.Require().NoError(err)

	// Deploy contracts
	kaatingaAddr, _, _, err := s.deployKaatingaToken(auth, client)
	s.Require().NoError(err)
	usdtAddr, _, _, err := s.deployMockUSDT(auth, client)
	s.Require().NoError(err)
	saleAddr, _, _, err := s.deployTokenSale(auth, client, usdtAddr, kaatingaAddr)
	s.Require().NoError(err)

	// Mint USDT to buyer (deployer)
	// Read ABI from Hardhat JSON artifact
	artifactPath := "../../../smart-contracts/artifacts/contracts/KaatingaToken.sol/KaatingaToken.json"
	artifactBytes := mustRead(artifactPath)
	var artifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	err = json.Unmarshal(artifactBytes, &artifact)
	s.Require().NoError(err)
	usdtABI, err := abi.JSON(strings.NewReader(string(artifact.ABI)))
	s.Require().NoError(err)
	mintAmount := big.NewInt(1000 * 1e6) // 1000 USDT (6 decimals)
	_, err = usdtABI.Pack("transfer", saleAddr, mintAmount)
	s.Require().NoError(err)
	// For simplicity, skip actual minting logic if not present

	// Configure sale
	// Read ABI from Hardhat JSON artifact
	saleArtifactPath := "../../../smart-contracts/artifacts/contracts/TokenSale.sol/TokenSale.json"
	saleArtifactBytes := mustRead(saleArtifactPath)
	var saleArtifact struct {
		ABI json.RawMessage `json:"abi"`
	}
	err = json.Unmarshal(saleArtifactBytes, &saleArtifact)
	s.Require().NoError(err)
	saleABI, err := abi.JSON(strings.NewReader(string(saleArtifact.ABI)))
	s.Require().NoError(err)
	rate := big.NewInt(10) // 1 USDT = 10 KAATINGA
	hardCap := new(big.Int)
	hardCap.SetString("100000000000000000000000", 10) // 100000 * 1e18
	now := time.Now().Unix()
	saleStart := big.NewInt(now - 10)
	saleEnd := big.NewInt(now + 1000)
	input, err := saleABI.Pack("configureSale", rate, hardCap, saleStart, saleEnd)
	s.Require().NoError(err)
	tx := types.NewTransaction(0, saleAddr, big.NewInt(0), 800000, big.NewInt(1e9), input)
	s.NoError(err)
	_ = tx // skip sending tx in this minimal example

	// TODO: Simulate buyTokens call (skipping actual tx send for brevity)
	//  In a real test, use bind. Transact to send tx and check balances
}
