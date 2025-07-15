package test

import (
	"context"
	"fmt"
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
	postgresHelper *bochka.Bochka[*bochka.PostgresService]

	printContainerLogs bool

	testContext       context.Context
	testContextCancel context.CancelFunc
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationSuite))
}

func (s *IntegrationSuite) SetupSuite() {
	ctx := context.Background()

	// 1. First ensure smart-contracts directory exists and has dependencies
	s.setupSmartContractsProject()

	// 2. Compile contracts (this happens during test)
	err := s.compileContracts()
	s.Require().NoError(err)

	s.printContainerLogs = true

	// 3. Start Anvil container with network alias

	network, err := bochka.NewNetwork(ctx)
	s.NoError(err)

	s.postgresHelper = bochka.NewPostgres(
		s.T(),
		ctx,
		bochka.WithCustomImage("timescale/timescaledb-ha", "pg17.4-ts2.18.2-oss"),
		bochka.WithNetwork(network),
	)
	if err = s.postgresHelper.Start(); err != nil {
		s.T().Fatalf("Failed to start Postgres: %v", err)
	}

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

	// 4. Deploy contracts to running Anvil
	_, _ = s.deployContracts()

	if s.printContainerLogs {
		s.postgresHelper.PrintLogs()
	}
}

func (s *IntegrationSuite) printLogs(ctx context.Context, container testcontainers.Container, name string) {
	logs, err := s.getContainerLogs(ctx, container)
	if err != nil {
		s.T().Errorf("failed to get %s container logs: %v", name, err)
		return
	}

	s.T().Logf("%s container logs:\n%s", name, logs)
}

func (s *IntegrationSuite) TearDownSuite() {
	ctx := context.Background()
	if s.AnvilContainer != nil {
		_ = s.AnvilContainer.Terminate(ctx)
	}
	if s.postgresHelper != nil {
		_ = s.postgresHelper.Close()
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
