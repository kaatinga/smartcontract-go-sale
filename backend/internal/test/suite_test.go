package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kaatinga/bochka"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Suite struct for integration tests
type IntegrationSuite struct {
	suite.Suite
	AnvilContainer testcontainers.Container
	AnvilEndpoint  string
	postgresImage  *bochka.Bochka

	testContext       context.Context
	testContextCancel context.CancelFunc
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

func (s *IntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// Create network first
	network, err := bochka.NewNetwork(ctx)
	s.NoError(err)

	// Start Anvil with network alias
	anvilReq := testcontainers.ContainerRequest{
		Image:        "anvil-socat",
		ExposedPorts: []string{"8545/tcp"},
		Cmd:          []string{"sh", "-c", "anvil --host 127.0.0.1 --port 8546 & socat TCP-LISTEN:8545,fork TCP:127.0.0.1:8546"},
		Networks:     []string{network.Name},
		NetworkAliases: map[string][]string{
			network.Name: {"anvil"},
		},
		WaitingFor: wait.ForListeningPort("8545/tcp"),
	}

	anvilC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: anvilReq,
		Started:          true,
	})
	s.NoError(err)
	s.AnvilContainer = anvilC

	// For external access (from host) - this is what your test needs
	anvilHost, err := anvilC.Host(ctx)
	s.NoError(err)
	anvilPort, err := anvilC.MappedPort(ctx, "8545")
	s.NoError(err)
	s.AnvilEndpoint = fmt.Sprintf("http://%s:%s", anvilHost, anvilPort.Port())

	s.T().Logf("Anvil endpoint: %s", s.AnvilEndpoint)
}

func (s *IntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.AnvilContainer != nil {
		_ = s.AnvilContainer.Terminate(ctx)
	}
	if s.postgresImage != nil {
		_ = s.postgresImage.Close()
	}
}

func (s *IntegrationSuite) SetupTest() {
	s.testContext, s.testContextCancel = context.WithTimeout(context.Background(), 10*time.Second)
}

func (s *IntegrationSuite) TearDownTest() {
	if s.testContextCancel != nil {
		s.testContextCancel()
	}
}

func (s *IntegrationSuite) getContainerLogs(ctx context.Context, container testcontainers.Container) ([]byte, error) {
	logReader, err := container.Logs(ctx)
	if err != nil {
		return nil, err
	}
	defer logReader.Close()

	return io.ReadAll(logReader)
}

func (s *IntegrationSuite) printLogs(ctx context.Context, container testcontainers.Container, name string) {
	logs, err := s.getContainerLogs(ctx, container)
	if err != nil {
		s.T().Errorf("failed to get %s container logs: %v", name, err)
		return
	}

	s.T().Logf("%s container logs:\n%s", name, logs)
}

func (s *IntegrationSuite) TestAnvilRPC() {
	// Print Anvil container logs for debugging
	s.printLogs(s.testContext, s.AnvilContainer, "Anvil")

	// TODO: Anvil is binding to 127.0.0.1 instead of 0.0.0.0 despite --host parameter
	// This is a known issue with the Foundry image. Need to find a workaround.
	// For now, just verify the container is running
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
