//go:build integration

package messaging_test

import (
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/fredsaggio/url-shortener/internal/messaging"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	rabbitMQUser     = "shortbit"
	rabbitMQPassword = "test-secret-password"
)

func startRabbitMQ(t *testing.T) (testcontainers.Container, string) {
	t.Helper()
	ctx := t.Context()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "rabbitmq:4-alpine",
			ExposedPorts: []string{"5672/tcp"},
			Env: map[string]string{
				"RABBITMQ_DEFAULT_USER": rabbitMQUser,
				"RABBITMQ_DEFAULT_PASS": rabbitMQPassword,
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

func rabbitMQURL(address string, password string) string {
	return (&url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(rabbitMQUser, password),
		Host:   address,
		Path:   "/",
	}).String()
}

func TestConnectIntegration(t *testing.T) {
	_, address := startRabbitMQ(t)

	t.Run("connects with valid credentials", func(t *testing.T) {
		conn, err := messaging.Connect(rabbitMQURL(address, rabbitMQPassword), "shortbit-test")
		if err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })

		if conn.IsClosed() {
			t.Fatal("Connect() returned a closed connection")
		}
	})

	t.Run("rejects invalid credentials without leaking them", func(t *testing.T) {
		wrongPassword := "wrong-secret-password"

		conn, err := messaging.Connect(rabbitMQURL(address, wrongPassword), "shortbit-test")
		if err == nil {
			_ = conn.Close()
			t.Fatal("Connect() error = nil, want authentication error")
		}

		if strings.Contains(err.Error(), wrongPassword) {
			t.Errorf("Connect() error leaks the password: %q", err)
		}
	})
}
