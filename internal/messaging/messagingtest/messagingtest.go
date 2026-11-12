//go:build integration

package messagingtest

import (
	"net"
	"net/url"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	rabbitMQImage = "rabbitmq:4-alpine"

	User     = "shortbit"
	Password = "test-secret-password"
)

// Start runs a disposable RabbitMQ and returns the container and its AMQP
// address (host:port).
func Start(t *testing.T) (testcontainers.Container, string) {
	t.Helper()
	ctx := t.Context()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        rabbitMQImage,
			ExposedPorts: []string{"5672/tcp"},
			Env: map[string]string{
				"RABBITMQ_DEFAULT_USER": User,
				"RABBITMQ_DEFAULT_PASS": Password,
			},
			WaitingFor: wait.ForLog("Server startup complete"),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start RabbitMQ container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get RabbitMQ host: %v", err)
	}

	port, err := container.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatalf("get RabbitMQ port: %v", err)
	}

	return container, net.JoinHostPort(host, port.Port())
}

func URL(address string, password string) string {
	return (&url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(User, password),
		Host:   address,
		Path:   "/",
	}).String()
}
