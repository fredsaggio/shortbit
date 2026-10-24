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

func TestConnectIntegration(t *testing.T) {
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

	address := net.JoinHostPort(host, port.Port())

	t.Run("connects with valid credentials", func(t *testing.T) {
		rabbitURL := (&url.URL{
			Scheme: "amqp",
			User:   url.UserPassword(rabbitMQUser, rabbitMQPassword),
			Host:   address,
			Path:   "/",
		}).String()

		conn, err := messaging.Connect(rabbitURL, "shortbit-test")
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
		rabbitURL := (&url.URL{
			Scheme: "amqp",
			User:   url.UserPassword(rabbitMQUser, wrongPassword),
			Host:   address,
			Path:   "/",
		}).String()

		conn, err := messaging.Connect(rabbitURL, "shortbit-test")
		if err == nil {
			_ = conn.Close()
			t.Fatal("Connect() error = nil, want authentication error")
		}

		if strings.Contains(err.Error(), wrongPassword) {
			t.Errorf("Connect() error leaks the password: %q", err)
		}
	})
}
